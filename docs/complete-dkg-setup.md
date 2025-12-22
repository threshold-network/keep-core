# How to Achieve Complete DKG

## Overview

DKG (Distributed Key Generation) requires **100 operators** to be selected from the sortition pool. To achieve complete DKG, you need multiple nodes running with different operators.

## DKG Requirements

### Hard Requirements

1. **Group Size**: 100 operators must be selected
   - Defined in contract: `EcdsaDkgValidator.groupSize = 100`
   - Cannot be changed without contract upgrade

2. **Active Threshold**: At least 90 operators must participate successfully
   - Defined: `activeThreshold = 90` (90% of groupSize)
   - This is the minimum for DKG to succeed

3. **Pre-Parameters**: Each operator needs pre-generated cryptographic parameters
   - Generated automatically when node starts
   - Must have at least as many pre-params as group members they control

### Why Single Operator Fails

With **1 operator**:
- Sortition pool selects 100 operators
- You only have 1 operator registered
- Even if selected, DKG protocol needs multiple operators to communicate
- **Result**: DKG cannot complete

## Solutions for Complete DKG

### Solution 1: Multiple Local Nodes (Recommended for Testing)

Run multiple `keep-client` nodes locally, each with:
- Different operator keyfile
- Different LibP2P port
- Different storage directory
- Different config file

#### Step 1: Create Multiple Config Files

**config.node1.toml:**
```toml
[ethereum]
URL = "http://localhost:8545"
KeyFile = "/path/to/operator1-keyfile"
KeyFilePassword = "password"

[network]
Port = 3919
Peers = []  # First node has no peers

[storage]
Dir = "/path/to/storage/node1"

[clientinfo]
Port = 9601
```

**config.node2.toml:**
```toml
[ethereum]
URL = "http://localhost:8545"
KeyFile = "/path/to/operator2-keyfile"
KeyFilePassword = "password"

[network]
Port = 3920
Peers = ["/ip4/127.0.0.1/tcp/3919/ipfs/<node1-peer-id>"]

[storage]
Dir = "/path/to/storage/node2"

[clientinfo]
Port = 9602
```

**config.node3.toml:**
```toml
[ethereum]
URL = "http://localhost:8545"
KeyFile = "/path/to/operator3-keyfile"
KeyFilePassword = "password"

[network]
Port = 3921
Peers = [
    "/ip4/127.0.0.1/tcp/3919/ipfs/<node1-peer-id>",
    "/ip4/127.0.0.1/tcp/3920/ipfs/<node2-peer-id>"
]

[storage]
Dir = "/path/to/storage/node3"

[clientinfo]
Port = 9603
```

#### Step 2: Generate Operator Keyfiles

For each node, create a new Ethereum account:

```bash
# Generate keyfile for node 1
geth account new --keystore /path/to/keystore

# Generate keyfile for node 2
geth account new --keystore /path/to/keystore

# ... repeat for all nodes
```

#### Step 3: Register and Authorize Operators

For each operator:

1. **Register in TokenStaking:**
   ```bash
   ./keep-client ethereum threshold token-staking register-staking-provider \
     --staking-provider <staking-provider-address> \
     --beneficiary <beneficiary-address> \
     --authorizer <authorizer-address> \
     --submit \
     --config configs/config.toml \
     --developer
   ```

2. **Authorize for WalletRegistry:**
   ```bash
   ./keep-client ethereum threshold token-staking authorize-application \
     --staking-provider <staking-provider-address> \
     --application <wallet-registry-address> \
     --amount <amount> \
     --submit \
     --config configs/config.toml \
     --developer
   ```

#### Step 4: Start Multiple Nodes

```bash
# Terminal 1: Node 1
KEEP_ETHEREUM_PASSWORD=password ./keep-client --config config.node1.toml start --developer

# Terminal 2: Node 2
KEEP_ETHEREUM_PASSWORD=password ./keep-client --config config.node2.toml start --developer

# Terminal 3: Node 3
KEEP_ETHEREUM_PASSWORD=password ./keep-client --config config.node3.toml start --developer

# ... start as many as you need (up to 100)
```

#### Step 5: Get Peer IDs

After starting each node, get its peer ID from logs:
```
Port: 3919
IPs : /ip4/127.0.0.1/tcp/3919/ipfs/16Uiu2HAmGsfKJaP4UGoGWYV6nxY8RPhVoHxT9rUQbPsxFedMHzEr
```

Use these peer IDs in other nodes' `Peers` configuration.

#### Step 6: Request New Wallet

Once all nodes are running and registered:

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit \
  --config configs/config.toml \
  --developer
```

All eligible operators will participate in DKG.

### Solution 2: Docker Compose Setup

Create a `docker-compose.yml` to run multiple nodes:

```yaml
version: '3.8'

services:
  keep-node-1:
    image: keep-client:latest
    environment:
      - KEEP_ETHEREUM_PASSWORD=password
      - LOG_LEVEL=info
    volumes:
      - ./config/node1.toml:/config.toml
      - ./storage/node1:/storage
    ports:
      - "3919:3919"
      - "9601:9601"
    command: start --config /config.toml --developer

  keep-node-2:
    image: keep-client:latest
    environment:
      - KEEP_ETHEREUM_PASSWORD=password
      - LOG_LEVEL=info
    volumes:
      - ./config/node2.toml:/config.toml
      - ./storage/node2:/storage
    ports:
      - "3920:3920"
      - "9602:9602"
    command: start --config /config.toml --developer
    depends_on:
      - keep-node-1

  # ... add more nodes as needed
```

### Solution 3: Testnet/Mainnet (Production)

For production DKG:

1. **Join Testnet/Mainnet**
   - Deploy contracts or use existing deployment
   - Register your operator
   - Authorize with sufficient stake

2. **Connect to Network**
   - Use bootstrap peers from network
   - Your node will discover other operators

3. **Wait for Selection**
   - When DKG is requested, sortition pool selects 100 operators
   - If your operator is selected, it will participate automatically

## Minimum Viable Setup

**For Testing (Not Production):**

You can test DKG with **fewer than 100 operators** if:
- All operators are registered and authorized
- All operators are selected (sortition pool has ≤100 operators)
- All operators can communicate via LibP2P
- Each operator has sufficient pre-parameters

**Example:** With 10 operators registered and all selected, DKG can complete.

## Verification Steps

### Check Operator Registration

```bash
# For each operator
OPERATOR_ADDR="<operator-address>"
./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
  --operator $OPERATOR_ADDR \
  --config configs/config.toml \
  --developer
```

### Check Authorization

```bash
STAKING_PROVIDER="<staking-provider-address>"
WALLET_REGISTRY="<wallet-registry-address>"
./keep-client ethereum threshold token-staking authorized-stake \
  --staking-provider $STAKING_PROVIDER \
  --application $WALLET_REGISTRY \
  --config configs/config.toml \
  --developer
```

### Monitor DKG Participation

```bash
# Check each node's logs
tail -f <node-log> | grep -i "dkg\|joining\|eligible"

# Check metrics from each node
curl -s http://localhost:9601/metrics | grep performance_dkg
curl -s http://localhost:9602/metrics | grep performance_dkg
# ... for all nodes
```

## Common Issues

### Issue: Not Enough Operators Selected

**Symptom:** DKG stuck in `AWAITING_RESULT`

**Solution:**
- Register more operators
- Ensure all operators are authorized
- Check sortition pool has enough operators

### Issue: Operators Can't Communicate

**Symptom:** DKG fails with network errors

**Solution:**
- Check LibP2P ports are open
- Verify peer IDs are correct in config
- Ensure nodes can reach each other

### Issue: Insufficient Pre-Parameters

**Symptom:** Log shows "pre-parameters pool size is too small"

**Solution:**
- Restart node (pre-params generated on startup)
- Wait for more pre-params to be generated
- Check node has been running long enough

## Automated Setup Scripts

### Quick Setup (Recommended)

Run the automated setup script:

```bash
# Setup 5 nodes (default)
./scripts/quick-dkg-setup.sh

# Or specify number of nodes
./scripts/quick-dkg-setup.sh 10
```

This script will:
1. Create operator keyfiles
2. Generate config files for each node
3. Create startup/stop/check scripts
4. Optionally register operators

**Important:** Operators must be registered before nodes can start. If nodes fail with "operator not registered", run:
```bash
./scripts/register-operators.sh 10
```

### Manual Setup Steps

If you prefer manual setup:

#### Step 1: Setup Nodes

```bash
# Create 10 nodes with default ports
./scripts/setup-multi-node-dkg.sh 10

# Custom ports
./scripts/setup-multi-node-dkg.sh 10 3919 9601 ./storage ./configs ./keystore
```

This creates:
- Operator keyfiles in `./keystore/`
- Config files in `./configs/node*.toml`
- Storage directories in `./storage/node*/`
- Startup scripts: `./configs/start-all-nodes.sh`
- Stop script: `./configs/stop-all-nodes.sh`
- Check script: `./configs/check-nodes.sh`

#### Step 2: Register Operators

```bash
# Interactive registration
./scripts/register-operators.sh 10

# Or manually register each operator (see script output)
```

#### Step 3: Start Nodes

```bash
# Start all nodes
./configs/start-all-nodes.sh

# Check status
./configs/check-nodes.sh
```

#### Step 4: Update Peer IDs

After nodes start, update peer IDs in configs:

```bash
./scripts/update-peer-ids.sh

# Restart nodes to apply peer connections
./configs/stop-all-nodes.sh
./configs/start-all-nodes.sh
```

#### Step 5: Trigger DKG

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer
```

## Summary

**To achieve complete DKG:**

1. ✅ **Multiple Operators**: Need 100 operators (or all registered if <100)
2. ✅ **Registration**: Each operator registered in TokenStaking
3. ✅ **Authorization**: Each operator authorized for WalletRegistry
4. ✅ **Network**: Operators can communicate via LibP2P
5. ✅ **Pre-Parameters**: Each operator has sufficient pre-params
6. ✅ **Selection**: Operators selected by sortition pool

**For Local Testing:**
- Run multiple nodes locally with different configs
- Register and authorize each operator
- Request new wallet to trigger DKG

**For Production:**
- Join testnet/mainnet
- Register and authorize your operator
- Wait to be selected for DKG rounds

## Related Documentation

- `docs/test-dkg-locally.md` - Basic DKG testing guide
- `docs/dkg-request-new-wallet-flow.md` - Complete DKG flow
- `docs/dkg-stuck-solutions.md` - Troubleshooting stuck DKG
- `docs/development/local-t-network.adoc` - Local network setup

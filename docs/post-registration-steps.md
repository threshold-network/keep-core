# Steps After Operator Registration

This guide covers what to do after successfully registering operators.

## Prerequisites

✅ All operators are registered and authorized  
✅ All operators have ETH for gas  
✅ All operators have T tokens staked  

## Step-by-Step Guide

### 1. Verify Registration

```bash
# Quick check - verify operators are in the pool
./configs/check-nodes.sh

# Detailed check for a specific operator
OPERATOR="0xef38534ea190856217cbaf454a582beb74b9e7bf"
./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
  --operator $OPERATOR \
  --config configs/config.toml \
  --developer
```

### 2. Start All Nodes

```bash
# Start all nodes (they won't connect yet - no peer IDs)
./configs/start-all-nodes.sh

# Wait for nodes to initialize
sleep 10
```

**Note:** Nodes will start but won't be able to connect to each other until peer IDs are configured.

### 3. Check Node Status

```bash
# Verify nodes are running
./configs/check-nodes.sh

# Check individual node diagnostics
curl -s http://localhost:9601/diagnostics | jq .
curl -s http://localhost:9602/diagnostics | jq .
# ... repeat for all nodes (ports 9601, 9602, 9603, etc.)
```

Look for:
- ✅ Node is running
- ✅ Operator address matches
- ✅ Peer ID is available (in diagnostics output)

### 4. Update Peer IDs

```bash
# Collect peer IDs from running nodes and update config files
./scripts/update-peer-ids.sh
```

This script:
- Queries each node's `/diagnostics` endpoint
- Extracts peer IDs
- Updates the `Peers` array in each config file

**Important:** Each node's config will be updated with peer IDs of other nodes.

### 5. Restart Nodes

```bash
# Stop all nodes
./configs/stop-all-nodes.sh

# Wait a moment
sleep 2

# Start again (now with peer IDs configured)
./configs/start-all-nodes.sh

# Wait for nodes to connect
sleep 10
```

### 6. Verify Connectivity

```bash
# Check nodes are connected
./configs/check-nodes.sh

# Check LibP2P metrics
curl -s http://localhost:9601/metrics | grep libp2p
curl -s http://localhost:9602/metrics | grep libp2p
# ... for all nodes
```

Look for:
- ✅ Multiple peers connected
- ✅ No connection errors in logs

### 7. Trigger DKG

```bash
# Request a new wallet (this triggers DKG)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit \
  --config configs/config.toml \
  --developer
```

This will:
- Create a DKG request
- Select operators from the sortition pool
- Initiate the DKG process

### 8. Monitor DKG Progress

```bash
# Use the monitoring script
./scripts/monitor-dkg.sh

# Or check manually
curl -s http://localhost:9601/diagnostics | jq '.DkgState'
curl -s http://localhost:9602/diagnostics | jq '.DkgState'
# ... for all nodes

# Watch logs for DKG activity
tail -f logs/node1.log | grep -i dkg
tail -f logs/node2.log | grep -i dkg
```

**DKG States:**
- `IDLE` (0) - No DKG in progress
- `AWAITING_SEED` (1) - Waiting for seed submission
- `AWAITING_RESULT` (2) - Waiting for DKG result
- `CHALLENGE` (3) - DKG result challenged

### 9. Verify DKG Completion

```bash
# Check if DKG completed successfully
./scripts/check-dkg-timing.sh

# Check for wallet creation
./keep-client ethereum ecdsa wallet-registry wallets \
  --config configs/config.toml \
  --developer
```

## Troubleshooting

### Nodes Won't Start

**Check:**
- Operators are registered: `./configs/check-nodes.sh`
- ETH balance is sufficient: `./scripts/fund-operators.sh`
- Config files are valid: Check for syntax errors

### Nodes Can't Connect

**Check:**
- Peer IDs are updated: `grep Peers configs/node*.toml`
- Ports are not blocked: `netstat -an | grep 3919`
- Nodes are restarted after updating peer IDs

### DKG Stuck

**Check:**
- All selected operators are running
- All operators can communicate (LibP2P connections)
- DKG timeout hasn't expired
- Check logs: `tail -f logs/node*.log | grep -i error`

### DKG Fails

**Common causes:**
- Not enough operators selected (need at least group size, typically 100)
- Operators can't communicate
- Insufficient authorization
- Network issues

**Solutions:**
- Register more operators
- Check LibP2P connectivity
- Verify authorization amounts
- Check network configuration

## Quick Reference

```bash
# Complete workflow (after registration)
./configs/start-all-nodes.sh          # Start nodes
sleep 10                               # Wait
./scripts/update-peer-ids.sh          # Update peer IDs
./configs/stop-all-nodes.sh           # Stop nodes
./configs/start-all-nodes.sh          # Restart with peer IDs
sleep 10                               # Wait for connections
KEEP_ETHEREUM_PASSWORD=password \
  ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer  # Trigger DKG
./scripts/monitor-dkg.sh              # Monitor progress
```

## Next Steps

After DKG completes successfully:
- ✅ Wallet is created and ready
- ✅ Operators can participate in signing
- ✅ System is ready for production use

For production deployment, ensure:
- All operators are properly registered
- Sufficient stake and authorization
- Network connectivity is stable
- Monitoring is in place


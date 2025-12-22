# What to Do After Nodes Have Started

This guide covers the steps to take after your `keep-client` nodes have successfully started.

## Current Status

✅ Nodes are running  
✅ Peer IDs have been extracted and updated in config files  
⏭️ Next: Restart nodes to establish connections

## Step-by-Step Guide

### 1. Verify Current Status

```bash
# Check which nodes are running
./configs/check-nodes.sh

# Check individual node diagnostics
curl -s http://localhost:9601/diagnostics | jq '.client_info'
curl -s http://localhost:9602/diagnostics | jq '.client_info'
```

### 2. Restart Nodes (Required for Peer Connections)

After updating peer IDs, nodes need to be restarted to establish LibP2P connections:

```bash
# Stop all nodes
./configs/stop-all-nodes.sh

# Wait a moment for cleanup
sleep 3

# Start all nodes again (now with peer IDs configured)
./configs/start-all-nodes.sh

# Wait for nodes to initialize and connect
sleep 10
```

### 3. Verify Peer Connectivity

```bash
# Check node status
./configs/check-nodes.sh

# Check connected peers for each node
curl -s http://localhost:9601/diagnostics | jq '.connected_peers | length'
curl -s http://localhost:9602/diagnostics | jq '.connected_peers | length'
curl -s http://localhost:9603/diagnostics | jq '.connected_peers | length'

# View connected peers
curl -s http://localhost:9601/diagnostics | jq '.connected_peers'
```

**Expected:** Each node should show other nodes in `connected_peers` array.

### 4. Check LibP2P Metrics

```bash
# Check connection metrics
curl -s http://localhost:9601/metrics | grep libp2p
curl -s http://localhost:9602/metrics | grep libp2p

# Look for:
# - connected_peers_count (should be > 0)
# - No connection errors
```

### 5. Verify Operator Registration

Before triggering DKG, ensure all operators are registered:

```bash
# Check registration status for all nodes
for i in {1..3}; do
  OPERATOR=$(curl -s http://localhost:960$i/diagnostics | jq -r '.client_info.chain_address')
  echo "Node $i operator: $OPERATOR"
  KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
    "$OPERATOR" --config configs/config.toml --developer 2>&1 | tail -1
done
```

### 6. Trigger DKG (Distributed Key Generation)

Once nodes are connected and operators are registered, trigger a DKG:

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

### 7. Monitor DKG Progress

```bash
# Watch logs for DKG activity
tail -f logs/node1.log | grep -i dkg
tail -f logs/node2.log | grep -i dkg
tail -f logs/node3.log | grep -i dkg

# Check DKG state via diagnostics (if available)
curl -s http://localhost:9601/diagnostics | jq '.DkgState // "not available"'
```

**DKG States:**
- `IDLE` (0) - No DKG in progress
- `AWAITING_SEED` (1) - Waiting for seed submission
- `AWAITING_RESULT` (2) - Waiting for DKG result
- `CHALLENGE` (3) - DKG result challenged

### 8. Verify DKG Completion

```bash
# Check for wallet creation
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry wallets \
  --config configs/config.toml \
  --developer

# Check logs for completion messages
grep -i "wallet.*created\|dkg.*complete" logs/node*.log
```

## Troubleshooting

### Nodes Won't Connect

**Symptoms:**
- `connected_peers` array is empty
- No LibP2P connections in metrics

**Solutions:**
1. Verify peer IDs are correct:
   ```bash
   grep Peers configs/node*.toml
   ```

2. Check network ports aren't blocked:
   ```bash
   netstat -an | grep -E "3919|3920|3921"
   ```

3. Ensure nodes were restarted after updating peer IDs

4. Check logs for connection errors:
   ```bash
   tail -f logs/node*.log | grep -i "connection\|peer\|libp2p"
   ```

### DKG Not Starting

**Check:**
- All selected operators are running
- Operators are registered in both RandomBeacon and WalletRegistry
- Sufficient authorization amounts
- Network connectivity is stable

### DKG Stuck

**Check:**
- All operators can communicate (LibP2P connections)
- DKG timeout hasn't expired
- Check logs for errors:
  ```bash
  tail -f logs/node*.log | grep -i error
  ```

## Quick Reference

```bash
# Complete workflow (after nodes started)
./scripts/update-peer-ids.sh          # Update peer IDs (already done)
./configs/stop-all-nodes.sh           # Stop nodes
./configs/start-all-nodes.sh          # Restart with peer IDs
sleep 10                               # Wait for connections
./configs/check-nodes.sh              # Verify status
curl -s http://localhost:9601/diagnostics | jq '.connected_peers | length'  # Check connections
KEEP_ETHEREUM_PASSWORD=password \
  ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
  --submit --config configs/config.toml --developer  # Trigger DKG
tail -f logs/node*.log | grep -i dkg   # Monitor DKG
```

## Next Steps After DKG Completes

Once DKG completes successfully:
- ✅ Wallet is created and ready
- ✅ Operators can participate in signing
- ✅ System is ready for use

For production deployment, ensure:
- All operators are properly registered
- Sufficient stake and authorization
- Network connectivity is stable
- Monitoring is in place


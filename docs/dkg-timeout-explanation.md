# DKG Timeout Explanation

## DKG Timeout Duration

The DKG timeout is **536 blocks**, not seconds. This is configured in the `WalletRegistry` contract:

```solidity
dkg.setResultSubmissionTimeout(536);
```

### Time Calculation

- **Block time**: In developer network, blocks are mined every ~10 seconds (varies by configuration)
- **Timeout duration**: 536 blocks × 10 seconds = **~5,360 seconds** ≈ **~89 minutes**
- **Note**: Actual block time depends on your Geth node configuration. Check block time with:
  ```bash
  # Check block mining rate
  BLOCK1=$(curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' http://localhost:8545 | jq -r '.result' | xargs -I {} printf "%d\n" {})
  sleep 10
  BLOCK2=$(curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' http://localhost:8545 | jq -r '.result' | xargs -I {} printf "%d\n" {})
  echo "Blocks in 10 seconds: $((BLOCK2 - BLOCK1))"
  ```

### Why 536 Blocks?

According to the contract comments, the timeout covers:
- 20 blocks to confirm the `DkgStarted` event off-chain
- 1 attempt of the off-chain protocol (216 blocks maximum)
- 3 blocks to submit the result for each of 100 members (3 × 100 = 300 blocks)
- **Total**: 20 + 216 + 300 = **536 blocks**

## Checking Timeout Status

### Current Status

```bash
# Check DKG state
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer

# Check if timed out
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer
```

### Understanding the Result

- **State `2` (AWAITING_RESULT)**: DKG is in progress
- **`has-dkg-timed-out` returns `false`**: DKG hasn't timed out yet
- **`has-dkg-timed-out` returns `true`**: DKG has timed out, you can notify timeout

## What to Do

### If DKG Hasn't Timed Out Yet

**Wait longer** - The timeout is ~9 minutes (536 seconds), not 5 minutes.

You can monitor progress:
```bash
# Monitor DKG progress
./scripts/wait-for-dkg-completion.sh 600  # Wait up to 10 minutes

# Or watch logs
tail -f logs/node*.log | grep -iE "dkg|keygen|result|submitted"
```

### If DKG Has Timed Out

Once `has-dkg-timed-out` returns `true`:

```bash
# Notify timeout to unlock the pool
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
  --submit --config configs/config.toml --developer

# Wait for state to reset
sleep 5

# Verify state is IDLE
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer
# Should return: 0
```

### If DKG Is Stuck Despite Connectivity

If nodes are connected but DKG isn't progressing:

1. **Check for connection issues:**
   ```bash
   tail -100 logs/node*.log | grep -iE "stream reset|ping.*fail|failed.*negotiate"
   ```

2. **Restart nodes** to refresh connections:
   ```bash
   ./configs/stop-all-nodes.sh
   sleep 3
   ./configs/start-all-nodes.sh
   sleep 10
   ```

3. **Wait for timeout** if DKG still doesn't progress

## Expected Timeline

- **0-20 blocks**: Confirming DkgStarted event
- **20-236 blocks**: Off-chain DKG protocol execution
- **236-536 blocks**: Result submission window
- **After 536 blocks**: Timeout can be notified

**Actual timeout duration depends on block time:**
- With 1-second blocks: ~9 minutes
- With 10-second blocks: ~89 minutes (typical for local dev)
- With 15-second blocks: ~134 minutes

**To check your block time:**
```bash
./scripts/check-dkg-timeout-details.sh
```

## Troubleshooting

### DKG Taking Longer Than Expected

**Possible causes:**
1. Network connectivity issues (check logs for "stream reset")
2. Operators not properly communicating
3. Insufficient operators selected
4. Block mining slower than expected

**Solutions:**
1. Check node connectivity: `curl -s http://localhost:9601/diagnostics | jq '.connected_peers | length'`
2. Check logs for errors: `tail -100 logs/node*.log | grep -i error`
3. Wait for full timeout period (~9 minutes)
4. If still stuck after timeout, notify timeout and restart

### Connection Issues

If you see "stream reset" or "ping test failed" errors:

1. **Restart nodes** to refresh LibP2P connections
2. **Verify peer configuration** is correct
3. **Check ports** aren't blocked
4. **Wait for connections to stabilize** before triggering new DKG

## Quick Reference

```bash
# Check timeout status
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer

# If true, notify timeout
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
  --submit --config configs/config.toml --developer

# Monitor progress
./scripts/wait-for-dkg-completion.sh 600  # 10 minutes max
```

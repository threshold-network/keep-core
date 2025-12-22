# DKG Timeout Actual Duration

## Important: Block Time Matters!

The DKG timeout is **536 blocks**, but the actual duration in minutes depends on your **block time**.

## Block Time Detection

Your local Geth node's block time determines how long 536 blocks actually takes:

```bash
# Check block mining rate
BLOCK1=$(curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545 | jq -r '.result' | xargs -I {} printf "%d\n" {})

sleep 10

BLOCK2=$(curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  http://localhost:8545 | jq -r '.result' | xargs -I {} printf "%d\n" {})

BLOCKS_MINED=$((BLOCK2 - BLOCK1))
BLOCK_TIME=$((10 / BLOCKS_MINED))

echo "Block time: ${BLOCK_TIME} seconds"
echo "536 blocks = $((536 * BLOCK_TIME)) seconds = $((536 * BLOCK_TIME / 60)) minutes"
```

## Common Block Times

| Block Time | 536 Blocks Duration |
|------------|---------------------|
| 1 second   | ~9 minutes          |
| 10 seconds | ~89 minutes         |
| 15 seconds | ~134 minutes        |

## Your Current Situation

Based on testing, your block time is **~10 seconds**, which means:
- **Timeout duration**: ~89 minutes
- **You've waited**: ~9 minutes  
- **Remaining**: ~80 minutes

## Why Timeout Calculation Includes Offset

The timeout formula is:
```
block.number > (startBlock + resultSubmissionStartBlockOffset + 536)
```

If a DKG result was challenged, `resultSubmissionStartBlockOffset` increases, extending the timeout further.

## What to Do

### Option 1: Wait for Timeout

Since you've already waited 9 minutes, you have ~80 minutes remaining. You can:

```bash
# Monitor periodically
watch -n 300 './scripts/check-dkg-state.sh'  # Check every 5 minutes

# Or use the wait script
./scripts/wait-for-dkg-completion.sh 5400  # Wait up to 90 minutes
```

### Option 2: Stop Nodes

Stop nodes to prevent participation (doesn't cancel on-chain state):

```bash
./configs/stop-all-nodes.sh
```

### Option 3: Check if Already Timed Out

Periodically check:

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry has-dkg-timed-out \
  --config configs/config.toml --developer
```

When it returns `true`, notify timeout:

```bash
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry notify-dkg-timeout \
  --submit --config configs/config.toml --developer
```

## Quick Check Script

Use the detailed timeout check:

```bash
./scripts/check-dkg-timeout-details.sh
```

This will show:
- Current state
- Current block number
- Whether timeout has occurred
- Block time information

## Summary

- **Your block time**: ~10 seconds
- **Actual timeout**: ~89 minutes (not 9 minutes!)
- **Remaining wait**: ~80 minutes
- **Action**: Wait longer or stop nodes to prevent participation

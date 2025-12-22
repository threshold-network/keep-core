# Multi-Node DKG Troubleshooting

## Common Issues

### Issue: Nodes Not Starting - "operator not registered"

**Error:**
```
FATAL: error initializing beacon: [could not set up sortition pool monitoring: [operator not registered for the staking provider]]
```

**Cause:** Operators must be registered in TokenStaking before nodes can start.

**Solution:**

1. **Register operators first:**
   ```bash
   ./scripts/register-operators.sh 10
   ```

2. **Then start nodes:**
   ```bash
   ./configs/start-all-nodes.sh
   ```

### Issue: Duplicate Config Keys

**Error:**
```
FATAL: toml: key BridgeAddress is already defined
```

**Cause:** Config file has duplicate entries.

**Solution:**
```bash
# Regenerate configs cleanly
rm -f configs/node*.toml
./scripts/setup-multi-node-dkg.sh 10
```

### Issue: Missing Bitcoin Electrum Config

**Error:**
```
FATAL: missing value for bitcoin.electrum.url
```

**Cause:** Bitcoin Electrum URL not configured.

**Solution:** The setup script now includes this automatically. If you see this error, regenerate configs:
```bash
./scripts/setup-multi-node-dkg.sh 10
```

### Issue: Missing Contract Addresses

**Error:**
```
FATAL: no contract code at given address
```

**Cause:** Missing contract addresses in config.

**Solution:** The setup script now includes all required contracts. Regenerate if needed:
```bash
./scripts/setup-multi-node-dkg.sh 10
```

## Correct Startup Order

1. **Setup nodes:**
   ```bash
   ./scripts/setup-multi-node-dkg.sh 10
   ```

2. **Register operators:**
   ```bash
   ./scripts/register-operators.sh 10
   ```

3. **Start nodes:**
   ```bash
   ./configs/start-all-nodes.sh
   ```

4. **Wait for startup (10-20 seconds), then check:**
   ```bash
   ./configs/check-nodes.sh
   ```

5. **Update peer IDs:**
   ```bash
   ./scripts/update-peer-ids.sh
   ```

6. **Restart nodes (to apply peer connections):**
   ```bash
   ./configs/stop-all-nodes.sh
   ./configs/start-all-nodes.sh
   ```

7. **Verify all nodes running:**
   ```bash
   ./configs/check-nodes.sh
   ```

8. **Trigger DKG:**
   ```bash
   KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry request-new-wallet \
     --submit --config configs/config.toml --developer
   ```

## Checking Node Logs

```bash
# Check specific node log
tail -f logs/node1.log

# Check for errors
grep -i "error\|fatal" logs/node*.log

# Check startup status
tail -20 logs/node1.log
```

## Verifying Operator Registration

```bash
# Get operator address from node config
OPERATOR=$(grep "KeyFile" configs/node1.toml | sed 's/.*--\(.*\)/\1/' | head -1)

# Check if registered
./keep-client ethereum ecdsa wallet-registry is-operator-in-pool \
  --operator "0x$OPERATOR" \
  --config configs/config.toml \
  --developer
```

## Complete Reset

If everything is broken:

```bash
# Stop all nodes
./configs/stop-all-nodes.sh

# Clean up
rm -rf logs/*.log logs/*.pid
rm -f configs/node*.toml

# Regenerate everything
./scripts/setup-multi-node-dkg.sh 10

# Register operators
./scripts/register-operators.sh 10

# Start nodes
./configs/start-all-nodes.sh
```

# "Notifications Not Supported" Error

## Error Message

```
ERROR subscription to event DkgResultSubmitted failed with error: [notifications not supported]
```

## What This Means

This error occurs when the client tries to subscribe to Ethereum events using **HTTP** instead of **WebSocket**.

**Why it happens:**
- Your `config.toml` uses `URL = "http://localhost:8545"` (HTTP)
- Event subscriptions require WebSocket connection (`ws://localhost:8546`)
- HTTP doesn't support push notifications/subscriptions

## Impact

**This is NOT a fatal error!**

The client will:
- ✅ Fall back to **polling** (checking events periodically)
- ✅ Continue working normally
- ✅ DKG will still proceed

**Trade-offs:**
- ⚠️ Slower event detection (polling vs real-time)
- ⚠️ More network requests (less efficient)
- ✅ Still fully functional for local development

## Solutions

### Option 1: Ignore It (Recommended for Local Dev)

This error is harmless. The client will work fine with polling. You can safely ignore it.

### Option 2: Use WebSocket (Optional)

If you want to eliminate the error, switch to WebSocket:

**Update `config.toml`:**
```toml
[ethereum]
URL = "ws://localhost:8546"  # Changed from http://localhost:8545
KeyFile = "..."
```

**Requirements:**
- Geth must be running with WebSocket enabled (`--ws`)
- WebSocket typically runs on port `8546` (vs HTTP on `8545`)

**Check if WebSocket is enabled:**
```bash
# Check Geth startup flags or config
# Should include: --ws --wsaddr "0.0.0.0" --wsport 8546
```

### Option 3: Suppress the Error (Not Recommended)

The error is informational - it tells you the client is using polling instead of subscriptions. Suppressing it would hide useful information.

## Verification

**Check if DKG is still working:**
```bash
# Check DKG state (should still work)
KEEP_ETHEREUM_PASSWORD=password ./keep-client ethereum ecdsa wallet-registry get-wallet-creation-state \
  --config configs/config.toml --developer

# Monitor DKG progress
./scripts/monitor-dkg.sh
```

**Check logs for DKG activity:**
```bash
tail -f logs/node*.log | grep -iE "dkg|keygen|member"
```

If you see DKG activity in logs, everything is working fine despite the error.

## Summary

**TL;DR:**
- ✅ **Safe to ignore** - Client falls back to polling
- ✅ **DKG still works** - Just uses polling instead of subscriptions
- ⚠️ **Optional fix** - Switch to WebSocket if you want real-time events
- 📝 **For local dev** - HTTP with polling is perfectly fine

The error is just informing you that the client is using a less efficient but still functional method to detect events.

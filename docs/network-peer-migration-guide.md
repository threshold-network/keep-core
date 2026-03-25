# Network Peer Migration Guide

## Quick Action

**If you have NOT manually configured `--network.peers` or `[LibP2P].Peers`**,
no action is needed. Your node uses embedded defaults that are updated
automatically with each client release.

**If you HAVE manually configured peers**, check your configuration for any
address containing `boar.network` or `staked.cloud` and remove it. The
recommended fix is to remove the `--network.peers` / `[LibP2P].Peers` setting
entirely so your node uses the new built-in defaults. See
[Required Action](#required-action-remove-hardcoded-boar-addresses) below.

> **Warning**: If you do nothing and your custom peer list contains only Boar
> addresses, your node will lose network connectivity when Boar infrastructure
> is decommissioned.

---

## Summary of Changes

The keep-client embedded peer list has been updated to replace Boar bootstrap
node addresses with curated operator-run peers.

The following changes are included in this release:

- **Boar bootstrap addresses removed** from embedded peer lists (mainnet and
  testnet). The Boar infrastructure (`bst-*.boar.network`) is being
  decommissioned.
- **New curated operator peers** are now embedded as defaults, hosted at
  `keep-nodes.io` (mainnet) and `test.keep-nodes.io` (testnet).
- **Firewall validation strengthened**: all peers are now validated through
  on-chain staking checks. Previously, embedded peer public keys bypassed
  staking validation via a firewall allow-list.
- **`--network.bootstrap` flag deprecated**: the flag still functions but will
  be removed in a future release.
- **Metric renamed**: `connected_bootstrap_count` has been renamed to
  `connected_wellknown_peers_count`.

Most operators do not need to take any action. If you have manually configured
`--network.peers` or `[LibP2P].Peers` in your configuration, read the next
section carefully.

---

## Required Action: Remove Hardcoded Boar Addresses

**If your node is configured with `--network.peers` (CLI flag) or
`[LibP2P].Peers` (configuration file), you must review and update your
configuration before Boar infrastructure is decommissioned.** Failure to act
will result in your node being unable to discover peers on the network.

### How to Check if You Are Affected

1. **Check your startup command** for the `--network.peers` flag:
   ```
   --network.peers=/dns4/bst-a01.tbtc.boar.network/tcp/3919/ipfs/16Uiu2HAm...
   ```

2. **Check your configuration file** (TOML, YAML, or JSON) for a `Peers`
   entry under the `[LibP2P]` section:
   ```toml
   [LibP2P]
     Peers = [
       "/dns4/bst-a01.tbtc.boar.network/tcp/3919/ipfs/16Uiu2HAm...",
       "/dns4/bst-b01.tbtc.boar.network/tcp/3919/ipfs/16Uiu2HAm..."
     ]
   ```

3. **If neither is set**, you are using the embedded defaults and no action is
   needed. The new operator peers are included automatically.

### Boar Addresses to Remove

Remove any peer address whose hostname contains `boar.network` or
`staked.cloud`. The specific Boar hostnames being decommissioned are:

| Network | Hostname |
|---------|----------|
| Mainnet | `bst-a01.tbtc.boar.network` |
| Mainnet | `bst-b01.tbtc.boar.network` |
| Testnet | `bst-a01.test.keep.boar.network` |

The peer ID suffix (the `/ipfs/16Uiu2HAm...` portion) varies, but the hostname
is the identifying part. Any multiaddress containing one of the hostnames above
must be removed.

### What to Do

**Option A (recommended):** Remove `--network.peers` / `[LibP2P].Peers`
entirely. Your node will then use the new embedded defaults, which are
maintained and updated with each client release.

Before:
```toml
[LibP2P]
  Peers = [
    "/dns4/bst-a01.tbtc.boar.network/tcp/3919/ipfs/16Uiu2HAm...",
    "/dns4/bst-b01.tbtc.boar.network/tcp/3919/ipfs/16Uiu2HAm..."
  ]
```

After:
```toml
[LibP2P]
  # Peers removed -- the client now uses built-in operator peer defaults.
```

Or, if using the CLI flag, simply remove `--network.peers` from your startup
command.

**Option B:** Replace the Boar addresses with currently active operator peer
addresses. Only use this option if you have a specific reason to maintain a
custom peer list.

### Why This Matters

When you set `--network.peers` (or `[LibP2P].Peers` in your configuration
file), the client uses your list exclusively and ignores the built-in defaults.

Internally, the `resolvePeers()` function in `config/peers.go` checks whether
`LibP2P.Peers` is already populated. If it is, the function returns immediately
without loading the embedded peer list. This means manually configured peers
completely override the defaults -- there is no merging.

If your custom peer list contains only Boar addresses, your node will be unable
to discover any peers once Boar infrastructure is decommissioned.

---

## Bootstrap Flag Deprecation

The `--network.bootstrap=true` flag is deprecated and will be removed in a
future release.

Historically, this flag marked a node as a bootstrap/relay node, which enabled
special behaviors such as adjusted dissemination timing and self-dial skipping.
As the network has matured, dedicated bootstrap mode is no longer necessary for
standard peer discovery.

The flag still functions and the node will log a deprecation warning when it is
used. Operators currently running with `--network.bootstrap=true` should plan
to stop using it. If you need to adjust dissemination timing directly, use the
`--network.disseminationTime` flag instead.

---

## Metric Rename

The `connected_bootstrap_count` metric has been renamed to
`connected_wellknown_peers_count`.

The metric semantics are unchanged -- it tracks the number of currently
connected well-known peers that are embedded in the client. Only the name has
changed to more accurately reflect that these peers are curated operator nodes
rather than traditional bootstrap nodes.

If you have Grafana dashboards, Prometheus alerts, or other monitoring that
references the old metric name, update your queries:

| Before | After |
|--------|-------|
| `connected_bootstrap_count` | `connected_wellknown_peers_count` |

The metric is exposed on the Client Info HTTP endpoint (port 9601 by default).

---

## Technical Background

This section provides additional context for operators who want to understand
the underlying mechanisms.

### Embedded Peer Resolution

The keep-client binary embeds a default peer list at compile time using Go's
`embed` package. The peer addresses are stored in the `config/_peers/` directory
with separate files for each network (mainnet, testnet).

When the client starts, `resolvePeers()` in `config/peers.go` runs the
following logic:

1. If `LibP2P.Peers` is already populated (via `--network.peers` flag or
   configuration file), return immediately without changes.
2. If the network type is `developer` or `unknown`, log a warning and return
   (no embedded defaults for these networks).
3. Otherwise, read the embedded peer list for the current network and set
   `LibP2P.Peers` to those values.

This design means that manually configured peers always take precedence over
embedded defaults. There is no merging of the two lists.

### Firewall Allow-List Change

Previously, the public keys of embedded peers were added to a firewall
allow-list, which allowed them to bypass the `IsRecognized()` on-chain staking
validation. This was a convenience for bootstrap infrastructure but weakened
the security model.

With this release, the allow-list is no longer populated with embedded peer
keys. All peers -- including the embedded operator peers -- must pass staking
validation through the on-chain contracts. This ensures that only properly
staked operators can participate in the network.

### Network Discovery Flow

The overall peer discovery architecture follows this path:

1. **Embedded peers** provide initial connectivity (the addresses in
   `config/_peers/<network>`).
2. **libp2p connections** are established to these well-known peers.
3. **DHT discovery** uses these initial connections to find additional peers
   across the network.
4. **Full mesh connectivity** is established as more peers are discovered.

Removing Boar addresses from the embedded list and replacing them with active
operator peers ensures that step 1 connects to reliable, staked infrastructure.

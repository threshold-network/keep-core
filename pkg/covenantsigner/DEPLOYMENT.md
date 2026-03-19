# Covenant Signer Deployment Topology

This document describes deployment topology constraints, coordination
mechanisms, and operational considerations for the covenant signer subsystem.
Source file references use `file:function` or `file:line` notation relative
to `pkg/covenantsigner/` and `pkg/tbtc/` within the repository root.

## 1. Expected Deployment Topology

The covenant signer is designed around a **single-node-per-wallet** deployment
model. Each covenant signer node controls the signing key shares for exactly
one wallet through its local `walletRegistry`.

When a signing request arrives, the engine calls `node.go:getSigningExecutor`
(line 319) to resolve the signing executor from the node's local wallet
registry. This function checks `walletRegistry.getSigners(walletPublicKey)`
(line 340) and returns `(nil, false, nil)` when the node holds no signer
shares for the requested wallet (lines 341-344), causing the engine to
reject the request without error (see Section 5 for details).

Each node runs its own HTTP server via `server.go:Initialize` (line 30),
binding to a configurable address and port (`server.go`, line 107). The
server maintains its own request store, authentication state, and signing
executor cache. No state is shared between nodes.

Multi-node deployments are possible when multiple nodes hold signer shares
for the same wallet, which is inherent to the threshold signing architecture.
However, this topology introduces coordination challenges documented in the
following sections.

## 2. Load Balancer Requirements

If multiple covenant signer nodes serve the same wallet and are placed behind
a single base URL, the load balancer **must use sticky sessions or
single-target routing**.

**Why this is required:**

Request deduplication is node-local (see Section 3 for full details). If a
load balancer distributes requests with the same `routeRequestID` across
different nodes, each node independently creates a new signing job for that
request, producing duplicate signing sessions for the same covenant
operation.

The Submit idempotency mechanism in `service.go:Submit` (line 254) checks
`store.GetByRouteRequest(route, routeRequestID)` to detect duplicate
requests. This lookup hits an in-memory map local to the process. A second
node has no visibility into the first node's store.

**Timeout considerations:**

The HTTP server is configured with a 30-second write timeout
(`server.go`, line 111). Load balancer health check intervals and upstream
timeout settings should account for this value to avoid premature connection
termination during signing operations.

**Authentication:**

Bearer token authentication (`server.go:withBearerAuth`, line 264) is
enforced for all non-loopback listen addresses. When running multiple nodes
behind the same load balancer endpoint, the `authToken` configuration must
be identical across all nodes.

## 3. Request Deduplication Scope

Request deduplication is **node-local only**. It prevents the same node from
creating multiple jobs for the same `routeRequestID`, but provides no
cross-node protection.

### Deduplication components

The deduplication logic in `service.go:Submit` (lines 253-266) relies on
three mechanisms:

1. **`Service.mutex`** (`service.go`, line 20): A `sync.Mutex` that
   serializes the check-and-insert critical section within `Submit()`. This
   is an in-process lock with no distributed coordination.

2. **`store.GetByRouteRequest()`** (`store.go`, line 152): Looks up existing
   jobs by `route + routeRequestID` in the `Store.byRouteKey` in-memory map
   (`store.go`, lines 17-18).

3. **`requestDigest` comparison** (`service.go`, line 258): Verifies payload
   consistency when a matching `routeRequestID` is found.

### Deduplication flow in `Submit()`

1. Acquire `s.mutex.Lock()` (line 253).
2. Call `s.store.GetByRouteRequest(route, input.RouteRequestID)` (line 254).
3. If found and digest matches: return the existing result idempotently
   (lines 264-265).
4. If found and digest differs: return an `inputError` indicating payload
   mismatch (lines 258-262).
5. If not found: create a new job, persist via `store.Put()` (line 301),
   then release the lock (line 305).

### Cross-node limitations

- The `sync.Mutex` is an in-process lock. Separate processes, even on the
  same host, maintain independent locks.
- The `Store` maps (`byRequestID`, `byRouteKey`) are in-memory per-process
  (`store.go`, lines 17-18).
- File persistence uses `persistence.BasicHandle`, which writes JSON files
  under `covenant-signer/jobs/` on the local filesystem with no cross-node
  synchronization.

**Consequence:** Multiple nodes behind a load balancer can produce duplicate
signing sessions for the same `routeRequestID` when requests are routed to
different nodes. This can trigger the P2P broadcast channel conflicts
described in Section 4.

## 4. P2P Signing Session Convergence

When a covenant signing request is accepted, the engine initiates a threshold
signing session over a P2P broadcast channel shared by all group members.
This section describes the signing flow and its behavior when multiple nodes
attempt concurrent signing for the same wallet.

### Signing flow

1. `covenantSignerEngine.submitSelfV1` / `submitQcV1`
   (`covenant_signer.go`, lines 206 / 272) obtain a `signingExecutor` and
   call `signBatch()`.

2. `signingExecutor.signBatch()` (`signing.go`, line 104) processes messages
   sequentially, calling `sign()` for each message in the batch.

3. `sign()` (`signing.go`, line 186) acquires a `semaphore.Weighted(1)` lock
   via `TryAcquire(1)` (line 191). This prevents concurrent signing for the
   same wallet on the same node. If the lock is not available, the call
   returns `errSigningExecutorBusy`.

4. Each signer controlled by the node runs a goroutine (`signing.go`,
   lines 238-403) that enters a retry loop with block-based coordination.

5. The P2P broadcast channel is wallet-scoped: all nodes holding signers for
   a given wallet share the channel named
   `{ProtocolName}-{walletPublicKeyHex}` (`node.go`, lines 351-355).

### Multi-node concurrent signing behavior

If two nodes receive the same signing request -- for example, due to load
balancer misconfiguration (Section 2) or missing cross-node deduplication
(Section 3) -- both attempt to initiate signing sessions on the same P2P
broadcast channel.

- The signing protocol uses an `announcer` and `signingDoneCheck` for group
  coordination (`signing.go`, lines 245-255). These mechanisms help members
  discover ongoing sessions and confirm completion.

- Threshold signing can converge if enough group members participate in a
  single session. However, conflicting concurrent sessions from different
  initiators may cause confusion in the broadcast channel, leading to wasted
  signing attempts or outright signing failures.

- The `semaphore.Weighted(1)` lock (`signing.go`, line 85) prevents a single
  node from running multiple signing sessions concurrently for the same
  wallet, but it does not coordinate across nodes.

### Retry and timing

- `signingAttemptsLimit = 5` (`node.go`, line 43) bounds each signer to a
  maximum of five retry attempts per message.
- `signingBatchInterludeBlocks = 2` (`signing.go`, line 36) inserts a
  2-block delay between sequential batch messages, giving signing done
  messages time to propagate across the broadcast channel before the next
  signing begins.

## 5. Wallet Ownership Guard

The covenant signer engine includes a wallet ownership check that prevents
nodes without signer shares from attempting to sign. This guard is
**necessary but not sufficient** for safe multi-node operation.

### How the guard works

Both `submitSelfV1()` (`covenant_signer.go`, line 220) and `submitQcV1()`
(`covenant_signer.go`, line 286) call
`cse.node.getSigningExecutor(walletPublicKey)`.

`getSigningExecutor()` (`node.go`, line 319) checks
`n.walletRegistry.getSigners(walletPublicKey)` (line 340). When
`len(signers) == 0`, the function returns `(nil, false, nil)` (lines
341-344), indicating the node does not control the requested wallet without
raising an error.

When the signing executor is not found, the engine returns
`ReasonPolicyRejected: "wallet is not controlled by this node"`
(`covenant_signer.go`, lines 224-225 and 290-291).

### Why this is necessary but not sufficient

**Necessary:** The guard prevents nodes that hold no signer shares for a
wallet from attempting to sign. Without it, any node receiving a request
could attempt to initiate a signing session, even if it has no key material
to contribute. This avoids unauthorized signing attempts and wasted
resources.

**Not sufficient:** In a threshold signing scheme, multiple nodes
legitimately hold signer shares for the same wallet, and
`getSigningExecutor` returns `true` for all of them. Without external
coordination -- such as sticky load balancer routing (Section 2), cross-node
request deduplication (Section 3), or an explicit leader election mechanism
-- multiple nodes may independently accept and begin processing the same
signing request, leading to the concurrent session conflicts described in
Section 4.

### Design assumption

The covenant signer is designed for a topology where signing requests for a
given wallet are directed to a single node that controls that wallet's
signing shares. External routing logic (load balancer configuration,
deployment topology, or application-level request routing) is expected to
maintain this invariant. The `getSigningExecutor` guard provides a safety net
against misconfigured routing but does not replace it.

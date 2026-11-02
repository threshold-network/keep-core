# G5 Go periphery triage (PR #4282 audit)

## Scope

Evidence note: the scan output named below is an internal run record and is not part of this record. The
`evidence/g5/pprof_secondcheck_test.go.txt` and `evidence/g5/pprof-dump/` scratch outputs referenced here are in
this tree (`evidence/`, committed with the report).

Scan run: (record not published; stamp `20261002-080105`, 5 passes, 14 leads).
Files: `pkg/clientinfo/performance.go`, `pkg/clientinfo/rpc_health.go`, `pkg/clientinfo/clientinfo.go`,
`cmd/flags.go`, `pkg/chain/ethereum/tbtc/gen/checkabi/main.go`, `pkg/crypto/ephemeral/private_key.go`,
`pkg/maintainer/btcdiff/bitcoin_difficulty.go`, `pkg/chain/ethereum/bitcoin_difficulty.go`,
`pkg/net/retransmission/ticker.go`, `pkg/net/libp2p/channel.go`, `pkg/net/libp2p/channel_manager.go`,
`pkg/net/local/broadcast_channel_manager.go`, `pkg/beacon/dkg/marshaling.go`,
`pkg/beacon/dkg/result/marshaling.go`, `pkg/beacon/registry/marshaling.go`,
`pkg/protocol/inactivity/marshaling.go`.
HEAD: the audit head (keep-core `reservations-epic` @ `49c4df13d`); PR diff base `a7ac8989`.
On-chain reference: `tbtc-v2@eec999aad:solidity/contracts/relay/LightRelay.sol`.

## Counts

Findings in this record: 5 (main findings F1, F2; supplement rows S2, S3, S6). Confirmed 4 (F1, F2, S2, S3),
Plausible 1 (S6).
Severity (Confirmed/Plausible only): Medium 1 (F1), Low 4 (F2, S2, S3, S6).

The remaining main finding and the supplement rows describe pre-existing code outside this PR's diff and
are reported privately. Leads: the per-source lead table is not published; the leads the fix plan references
are listed in the index's appendix.

## Summary table

| ID | Severity | Verdict | Origin | Title | Location |
|---|---|---|---|---|---|
| G5-S2 | Low | Confirmed | Introduced by PR #4282 | An in-flight local broadcast retransmit can cross into the next session's channels after release | `local.getBroadcastChannel/broadcastMessage` (`pkg/net/local/broadcast_channel_manager.go:45-46,66-73,84-93`) |
| G5-S3 | Low | Confirmed | Introduced by PR #4282 | The 1.2x gas margin truncates to 0% for gas estimates of 4 units or less | `ethereum.gasEstimateWithMargin` (`pkg/chain/ethereum/ethereum.go:537-540`, used at `pkg/chain/ethereum/bitcoin_difficulty.go:298`) |
| G5-S6 | Low | Plausible | Introduced by PR #4282 | A deserialized signer private-key share has no curve-order bound before signature use | `dkg.ThresholdSigner.Unmarshal` (`pkg/beacon/dkg/marshaling.go:52,65`) |
| G5-F1 | Medium | Confirmed | Introduced by PR #4282 | EnablePprof=false still exposes /debug/pprof/* on the metrics port | `pkg/clientinfo/clientinfo.go:6,49-58` |
| G5-F2 | Low | Confirmed | Introduced by PR #4282 | Enabling the documented pprof flag panics the node at startup | `pkg/clientinfo/clientinfo.go:69-75` |

## G5-F1: EnablePprof=false still exposes /debug/pprof/* on the metrics port

Confirmed, Medium (recalibrated from High; see the severity execution check below). `pkg/clientinfo/clientinfo.go:6`
imports `net/http/pprof` by name. The standard library pprof package runs an `init()` that registers
`/debug/pprof/`, `/cmdline`, `/profile`, `/symbol`, `/trace` on `http.DefaultServeMux` unconditionally
(GOROOT `src/net/http/pprof/pprof.go:93-101`). keep-common's `EnableServer` (`keep-common v1.7.1-tlabs.0`,
`pkg/clientinfo/clientinfo.go:48-70`) starts `&http.Server{Addr: ":" + port}` with a nil Handler, which
serves `DefaultServeMux`. Result: whenever `ClientInfo.Port != 0`, the profiling endpoints are live on that
port, whether `EnablePprof` is true or false.

The code comment at `clientinfo.go:50-55` claims that explicit registration "avoids the side-effecting blank
import of net/http/pprof". That is wrong: a named import runs the package `init()` exactly like a blank
import; only not importing the package at all avoids it. So the "gate" does not exist, and the flag only
decides whether a second, fatal registration attempt happens (G5-F2).

Proof (scratch program, since deleted): a standalone module that imports `net/http/pprof`, does nothing else
(flag-off path), and serves `http.ListenAndServe` on DefaultServeMux returned 200 for GET `/debug/pprof/`,
`/debug/pprof/cmdline`, and `/debug/pprof/heap` in both default GODEBUG mode and `GODEBUG=httpmuxgo121=1`.
Reproduced the keep-common wiring verbatim: no handler registration at all on the application side, yet the
pprof paths are served.

Who can reach it: the metrics port is operator-configured via `clientInfo.port` (default 9601,
`cmd/flags.go:255-261`) and binds all interfaces (`":port"` in keep-common `EnableServer`). The PR's own new
`docs/profiling.md:14-19` states the server "binds to all interfaces (0.0.0.0)" and that `EnablePprof` "is
the only thing that prevents the /debug/pprof/ endpoints from being reachable on that port". That claim is
false. Anyone who can reach the operator's 9601 can pull the process command line, symbol table, CPU
profiles, heap/goroutine dumps, and execution traces (see the severity execution check below for exactly
what that does and does not expose). There is no `EXPOSE` directive in the repo `Dockerfile` for this port
(`grep -c EXPOSE Dockerfile` = 0) and the k8s chart's `maintainer-service.yaml` publishes it only as a
`ClusterIP` service (`infrastructure/kube/templates/keep-maintainer/maintainer-service.yaml:6`), so by
default it is reachable from scrapers and co-tenant pods inside the same cluster/network, not from the
public internet; a bare-metal/VM operator who does not additionally firewall the host (the docs' own
recommendation, `docs/profiling.md:24-26`) exposes it to anyone on that host's network. The same path runs
in both `keep start` (`cmd/start.go`) and the experimental maintainer (`cmd/maintainer.go:116`).

Fix: remove the named `net/http/pprof` import entirely (so `init()` never runs); construct a private
`http.ServeMux` in keep-common's `EnableServer` and pass it to `http.Server.Handler`, then conditionally
mount pprof handlers on that private mux when `EnablePprof` is true. Until keep-common accepts a custom
handler, the minimal in-repo mitigation is to delete the import and the `registerPprofHandlers` calls and
accept that pprof is unavailable; `docs/profiling.md` must be corrected either way.

Second check: re-ran a fresh scratch program (standalone module, deleted after run): `import _ "net/http/pprof"`
plus a `net.Listen` + `&http.Server{Handler: nil}` serving `DefaultServeMux`, mirroring keep-common's wiring
exactly with zero application-side handler registration. Result: `GET /debug/pprof/ -> 200`,
`GET /debug/pprof/cmdline -> 200`, `GET /debug/pprof/heap -> 200`. This independently reproduces the claim
under go1.25.6 without relying on the deleted program.

Default bind address and port: `cmd/flags.go:255-261` sets `clientInfo.port` default to `9601`; keep-common's
`EnableServer` builds `&http.Server{Addr: ":" + port}`, which binds all interfaces (0.0.0.0), not localhost.
Both `keep start` (`cmd/start.go`) and `keep maintainer` (`cmd/maintainer.go:116-118`) call
`clientinfo.Initialize` on the same `ClientInfo.Port` config, so the exposure is identical in both processes.

Whether the PR introduced the named import: `git diff a7ac8989 49c4df13d -- pkg/clientinfo/clientinfo.go`
shows the base file had **no** `net/http/pprof` import of any kind (confirmed separately:
`git grep -l 'net/http/pprof' a7ac8989` across the whole base tree returns nothing) - the PR adds both
`"net/http"` and `"net/http/pprof"` as fresh named imports, plus `EnablePprof`, `registerPprofHandlers`, and
the comment at clientinfo.go:50-55/67-69. The assignment's framing ("it replaced a blank import") does not
match the diff: there was no blank import to replace; the comment describes a *hypothetical* blank import
("in place of the blank import ... that would otherwise register") as its justification for using a named
import instead, but a named import triggers the identical `init()` side effect as a blank one - the
comment's premise is the PR author's own mistaken belief about Go import semantics, confirmed wrong by the
scratch run above. The named import, and therefore the always-on exposure, is 100% PR-introduced.

Heap/goroutine dumps and key material (executed, see the severity check below): the earlier reading that
`GET /debug/pprof/heap?debug=1` "serializes all reachable heap objects including live protocol state" is
wrong and is corrected by execution. Go's heap profile is an allocation-site sampling profile (stack traces
plus byte counts of samples taken at allocation time), not a dump of the live object graph; there is no
pprof endpoint that serializes the contents of in-memory values. With an ephemeral private key, a 32-byte
stand-in TSS key share, and a symmetric session key held live in the test process, none of the 12
`/debug/pprof/*` endpoints (including `heap`, `heap?debug=1`, `goroutine?debug=1`, `goroutine?debug=2`,
`allocs`, `profile`, `trace`) contained the secret bytes in hex or raw form. `/debug/pprof/goroutine?debug=2`
dumps full stack traces (function names and call sites, revealing which protocol phase/goroutine shape is
active, but not key bytes or other local variable values). `/debug/pprof/cmdline` leaks the full process
command line (binary path and flags, e.g. config file path), which is a real but minor information leak, not
key material.

Second check (severity execution): built a worktree test (`pkg/clientinfo/pprof_secondcheck_test.go`,
`go test -v -run TestPprofSecondCheck ./pkg/clientinfo/`) that holds an ephemeral `btcec` private key, a
32-byte stand-in TSS key share, and a symmetric session key live in the process, serves the real
`clientinfo.Initialize(ctx, Config{Port: 39117, EnablePprof: false})`, and fetches all 12 `/debug/pprof/*`
paths plus two cost-measurement requests. Result: every path returns 200 (exposure proven) and none contain
the secret bytes in hex or raw form ("secrets-exposed=none" on every line, bodies 15 to 20951 bytes). Cost
measurement (same test, same running server): `GET /debug/pprof/profile?seconds=5` returned 200/396 bytes in
5.0043s; `GET /debug/pprof/trace?seconds=2` returned 200/12649 bytes in 2.0035s - confirming each call
blocks one server goroutine for exactly the caller-chosen duration, with nothing in
`Initialize`/keep-common's `EnableServer` rate-limiting, deduplicating, or capping concurrent calls, so a
remote caller can hold handlers open indefinitely by repeating or parallelizing `?seconds=N` requests.
Recalibration: the original High rested on an unexecuted "heap dumps may hold ephemeral protocol key
material" claim; execution disproves that specific mechanism (Go heap profiles are allocation-stack samples,
not object-graph dumps - see the corrected paragraph above), so the realistic impact is operational
information disclosure (cmdline, symbol table, active-goroutine shape) plus an unauthenticated, repeatable
CPU/trace cost on the node - not key exfiltration. Likelihood is bounded by network reachability to the
operator's 9601, which defaults to cluster-internal (k8s ClusterIP) or host-network (bare metal, firewall
recommended by the docs), not the public internet. Impact (moderate: info disclosure + minor repeatable CPU
cost) x likelihood (moderate: reachable by anyone already positioned on the operator's internal network or
co-tenant infrastructure, not an open-internet attacker) recalibrates this from High to **Medium**. Verdict
stays Confirmed; severity changes to Medium.

## G5-F2: Enabling the documented pprof flag panics the node at startup

Confirmed, Low. `Initialize` calls `registerPprofHandlers()` (`clientinfo.go:56`) when `EnablePprof` is true,
which re-registers the same five patterns on `DefaultServeMux` that the pprof package's `init()` already
registered (`clientinfo.go:69-75`). `http.HandleFunc` on a conflicting pattern panics, nothing in the call
chain (`initializeClientInfo`, `cmd/start.go:86`) recovers, so the process dies at startup.

Proof (scratch program, since deleted): replicated the five `http.HandleFunc` calls after importing
`net/http/pprof`. Default GODEBUG: `PANIC: pattern "/debug/pprof/cmdline" ... conflicts with pattern
"GET /debug/pprof/"` (Go 1.21+ mux method-prefixing). `GODEBUG=httpmuxgo121=1`: `PANIC: http: multiple
registrations for /debug/pprof/`. Both modes crash.

Evidence check: re-ran the panic both as a standalone reproduction of `registerPprofHandlers` (`go run` a
scratch program with the same five `http.HandleFunc` calls after `import "net/http/pprof"`) and as the real
production call path (`clientinfo.Initialize(ctx, Config{Port: 39118, EnablePprof: true})` in a throwaway
`go run`). Both panic identically in both GODEBUG modes: default - `panic: pattern "/debug/pprof/cmdline"
(registered at .../clientinfo.go:71) conflicts with pattern "GET /debug/pprof/" (registered at
.../pprof.go:100)`; `GODEBUG=httpmuxgo121=1` - `panic: http: multiple registrations for /debug/pprof/`.
Confirms G5-F2 end to end through the actual `Initialize` entry point, not just a mirrored reproduction.

Impact: an operator who follows `docs/profiling.md:32-43` (set `EnablePprof: true`, restart) finds the node
will not start; the documented escape hatch ("set it back to false when finished", `docs/profiling.md:42`)
masks that it was never usable. No attacker trigger: this is self-inflicted operator availability when
enabling a documented flag, and it couples with G5-F1: because the flag is inert when off (F1) and fatal when
on (F2), there is no configuration in which pprof is actually gated.

Fix: same as G5-F1 (private mux, mount handlers conditionally on it, drop the import so `init()` never
registers on `DefaultServeMux`).

## Scan deviations (stated in the report so they are on record)

- The record has since been corrected to the truth: `scope.tsv` records `pass_3_agents 11/12` and
  `pass_4_agents 10/12` (pass 3's agent 6 (periphery) returned an empty result, and pass 4's agent 4
  (execution-trace) and agent 10 (numerical-gap) returned only placeholder text, so their outputs were not
  captured into the passes' raw records). The `run-3.md`/`run-4.md` headers were corrected to 11/12 and
  10/12 respectively, and `full-report.md` was re-assembled from the unchanged run files. Per-finding "seen
  in N/5 runs" tallies were already computed from the run files, which are unchanged.
- A supplementary run re-covered the three lost specialties over the same 16-file scope: periphery (agent 6),
  execution-trace (agent 4), numerical-gap (agent 10). It ran outside the 5-pass loop; the ledger was not
  modified by it. New items are triaged in the "Supplementary run" and "New items" sections of this file
  (G5-S1 through G5-S8, 8 new items, 7 rejected, 17 duplicates of the items above). One agent-6 claim - that
  the two pprof findings no longer reproduce at HEAD - was refuted by execution (both still reproduce; see
  the "Refuted record correction" section of this file).
- The orchestrator demoted the `clientinfo.RPCHealthChecker.checkBitcoinHealth` stale-health-signal item
  from FINDING to LEAD after reading `pkg/clientinfo/rpc_health_test.go:399-446` directly: the test asserts
  the unhealthy gauge outcome as deliberate ("Health state semantics are unchanged"), so the item is a
  design decision, not an oversight.
- The orchestrator removed a `RetargetWithRefund` "missing-proxy-method" lead it proved wrong. Re-checked:
  the removal is correct; `LightRelayMaintainerProxy` exposes `retarget` (`LightRelayMaintainerProxy.sol:137`)
  and the Go `RetargetGasEstimate` binding is an `eth_estimateGas` against that method
  (`contract/LightRelayMaintainerProxy.go:628-643`), so no proxy method is missing.

## Proof programs used (all since deleted)

1. Scratch module mimicking keep-common's nil-handler server: `import _ "net/http/pprof"` +
   `http.ListenAndServe(port, nil)`. With `EnablePprof=false` semantics (no application-side handler
   registration), GET `/debug/pprof/`, `/cmdline`, `/heap` all returned 200 in both default GODEBUG and
   `GODEBUG=httpmuxgo121=1`. Proves G5-F1.
2. Scratch module reproducing the five `http.HandleFunc` calls from `registerPprofHandlers` after the pprof
   import. Default GODEBUG: `PANIC ... conflicts with pattern "GET /debug/pprof/"`; `httpmuxgo121=1`:
   `PANIC: http: multiple registrations for /debug/pprof/`. Proves G5-F2 in both mux modes.
3. `pkg/clientinfo/pprof_secondcheck_test.go` (saved under `evidence/g5/pprof_secondcheck_test.go`):
   in-package test holding live ephemeral-key/TSS-share/symmetric-key secrets while serving the real
   `Initialize(EnablePprof: false)`; fetches all 12 `/debug/pprof/*` paths plus two cost-measurement requests
   (`profile?seconds=5`, `trace?seconds=2`) against the same running server, and scans each response for the
   secret bytes (hex and raw). All 200, no secrets found; cost calls each block for exactly the requested
   duration. Re-proves G5-F1's exposure and grounds the severity recalibration.
4. A standalone `go run` of `clientinfo.Initialize(ctx, Config{EnablePprof: true})` against the worktree's
   real package (not a mirrored reproduction): panics identically to item 2 in both GODEBUG modes. Re-proves
   G5-F2 through the production entry point; evidence quoted inline in the G5-F2 section above.

Stdlib reference: Go `src/net/http/pprof/pprof.go:93-101` (`init()` registers all five patterns,
method-prefixed under default `httpmuxgo121=0`).

## Supplementary run (lost agents)

Pass 3 agent 6 (periphery) returned an empty result; pass 4 agents 4 (execution-trace) and 10 (numerical-gap)
returned placeholder text. This supplement re-ran those three specialties over the same 16-file scope,
outside the 5-pass loop (the ledger and full report were not modified); record counts were corrected to
11/12 and 10/12.

## Counts

- New items triaged: **3 in this record** (S2, S3, S6); the S rows that describe pre-existing code are
  reported privately and are not part of this record.
- Duplicates of the items above: 17 raw re-traces mapped to already-triaged IDs; one of them (agent-10
  "unbounded-probe-duration") is the recorded `uncancellable-context` key under a new label.
- Refuted record correction: **1** (the agent-6 claim that the two pprof findings no longer reproduce;
  refuted by execution).

Note: the triaged S rows are in the main Summary table above (this section keeps the per-item detail).

## New items

### G5-S2: An in-flight local broadcast retransmit can cross into the next session's channels after release

Confirmed, Low. `getBroadcastChannel` creates a per-creation retransmission ticker rooted in a cancel tracked
per channel name (`tickerCtx, cancelTicker := context.WithCancel(context.Background())`,
`pkg/net/local/broadcast_channel_manager.go:45-46`), and `broadcastMessage` delivers by name-keyed lookup of
the channel list (:66-73). `ReleaseBroadcastChannel` cancels the tickers and deletes the name's registry
entries (:84-93) but the cancel is not a barrier: a ticker already mid-fire inside `broadcastMessage`
re-looks-up the name's channel list, which now holds the next session's channels, and delivers the old
session's message (carrying the old sender transport ID and sequence number) into them. The receiver side's
dedup cache (`pkg/net/retransmission/retransmission.go:95-113`) records the foreign `id-seq` as seen. The
cross-session re-delivery mechanism is confirmed by read; the downstream "the next session's own retransmit
is dropped" consequence depends on a sequence-number collision and is [INFERENCE], not executed. Bounded to
the local network provider, which is wired only into test support (`pkg/internal/entrytest`,
`pkg/internal/dkgtest`), not the live libp2p deployment. The per-name cancel registry and release are
PR-introduced (the base rooted the ticker in `context.Background()` with no release path), so the residual
race lives in the PR's new mechanism. Fix: add a release barrier so an in-flight `broadcastMessage` cannot
complete against a post-release registry entry, and root the ticker in the caller's context.

### G5-S3: The 1.2x gas margin truncates to 0% for gas estimates of 4 units or less

Confirmed, Low. `gasEstimateWithMargin` (`pkg/chain/ethereum/ethereum.go:537-540`) is
`return uint64(float64(gasEstimate) * 1.2)`. `1.2` is stored as `~1.19999...` in a float64 and the `uint64()`
cast truncates toward zero, so the "20% margin" is actually a floor-only margin that lands at 0% for small
estimates. Executed against the verbatim helper under go1.25.6: g=1..4 -> no margin added at all; g=6 ->
16.7%; g=13 -> 15.4%; g=54000 -> 64800 (full 20%). For the real caller `RetargetWithRefund` the estimate is
`retargetGasOffset` (default 54000) plus call overhead, far above 4, so the reachable impact of the truncated
floor on the live retarget path is marginal - the defect is confirmed at the function level, not on a hot
real-world value. PR-introduced: the helper is absent at `a7ac8989` (the PR added it to cover
reimbursement-flow under-estimates, per the comment at :532-536). Fix: integer arithmetic -
`gasEstimate + gasEstimate/5` (or `+ (gasEstimate+4)/5` for a true 20% ceiling).

### G5-S6: A deserialized signer private-key share has no curve-order bound before signature use

Plausible, Low. `dkg.ThresholdSigner.Unmarshal` (`pkg/beacon/dkg/marshaling.go:52`) converts the
`GroupPrivateKeyShare` decimal string via `privateKeyShare.SetString(pbThresholdSigner.GroupPrivateKeyShare,
10)` (:65) and rejects only non-decimal input; the resulting `big.Int` carries no range check against the
curve order before it reaches `CalculateSignatureShare` (`signer.go:63-65`). A forged peer-delivered share
outside the subgroup then produces signature shares that do not verify. This is a variant of the recorded
member-index-truncation item in the same `Unmarshal`, on a different field (the share string, not the member
index). Precondition: wire reachability of the Membership message is unconfirmed - no `SetUnmarshaler`
registers `registry.Membership`/`dkg.ThresholdSigner` on any broadcast channel (the same reasoning that
demotes the index-truncation item to local-storage-only). PR-introduced file. Fix: reject a share whose
absolute value exceeds the curve order (or the group-size bound) after `SetString`.

## Refuted record correction

An agent reported that the two recorded pprof findings (`clientinfo.Initialize|pprof-unconditional-exposure`
= G5-F1, `clientinfo.registerPprofHandlers|pprof-handler-double-registration-panic` = G5-F2) "do not
reproduce at repo HEAD" because commit `d6dc72ea2` removed the blank import and replaced it with the named
import plus the gated `registerPprofHandlers` call. That correction is wrong: a named import of
`net/http/pprof` still runs the package's `init()`, which registers `/debug/pprof/*` on
`http.DefaultServeMux` unconditionally - the gate at `clientinfo.go:49-58` never reaches the registration the
`init()` already performed. `d6dc72ea2` changed the code but did not change the semantics the findings
describe. Both findings therefore stand as confirmed (G5-F1/G5-F2). Executed at HEAD (go1.25.6, scratch
module, since deleted): flag-off path (named import present, `registerPprofHandlers` never invoked) - `GET
/debug/pprof/`, `/cmdline`, `/heap` all return 200 on a nil-handler server; flag-on path (named import +
`registerPprofHandlers`) - panics at startup with `pattern "/debug/pprof/cmdline" ... conflicts with pattern
"GET /debug/pprof/"`. This is exactly the class of mistaken Go-import-semantics that the shared rules warn
about; the execution, not the commit-message reading, is the ground truth.

## Verification notes

- All new items were gated against the repo files and the pinned on-chain guards. The gas-margin truncation
  (G5-S3) and both pprof paths (record correction) were verified by execution under go1.25.6 (scratch
  modules, since deleted); the remaining items are confirmed by direct code read with the specific
  unverified preconditions named above.
- Raw agent outputs cited bundle-relative line numbers; the repo line numbers above were re-pinned by reading
  the live files at the audit head.
- No tracked source file was modified. The scan ledger and full report are unchanged by this supplement.
  Scratch modules and the temporary supplement bundle were cleaned up at the end of the task.
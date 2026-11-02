# G4 triage: go-node-protocol (scan `20261002-075759`)

Evidence note: the scan output (`full-report.md`) named in this file is an internal run record and is not
part of this record. The `evidence/g4/` scratch tests referenced here are in this tree (`evidence/`,
committed with the report).

Repo: PR #4282 audit branch, code at `49c4df13d`, PR diff `a7ac8989..49c4df13d`.
Scope: 18 Go files (tbtc node/coordination/wallet/moving-funds/redemption/transaction-monitor,
tecdsa DKG and signing protocol + marshaling, cmd start/maintainer, beacon node, libp2p).

Proof programs (scratch Go programs, run outside the repo and deleted after use):
- Scratch program simulating the metrics window map (insert-then-cleanup at 100 windows): reproduces a
  nil-deref in the coordination window metrics (a pre-existing path, not part of this record).
- Scratch program with a bare goroutine whose payload panics, no recover anywhere: the process exits with
  code 2, `runtime error: invalid memory address or nil pointer dereference`. Proves the crash semantics the
  Disputed G4-F7 and the pre-existing no-recover findings rest on: a panic in any of these goroutines kills
  the whole node.

## Counts

Findings in this record: 8 - Confirmed 5 (F6, F8, F9, F10, F11), Disputed 2 (F7, F22), False
positive 1 (F13). Other rows in the same triage describe pre-existing code outside this PR's diff; those
are reported privately and are not part of this record.
Severity (Confirmed/Plausible only): Medium 2 (F8, F9), Low 3 (F6, F10, F11).

Leads: the per-source lead table is not published; the leads the fix plan references are listed in the
index's appendix.

## Summary table

| ID | Severity | Verdict | Origin | Title | Location |
|----|----------|---------|--------|-------|----------|
| G4-F6 | Low | Confirmed | PR #4282 | Reservation flag asymmetry strands anchors | cmd/start.go:236-243 (paired-flag warn-only branch) |
| G4-F7 | - | Disputed | PR #4282 | Failed watcher wiring kills whole node | reservation_wiring.go:234-238 (WireReservationWatchers nil-chain guards) |
| G4-F8 | Medium | Confirmed | PR #4282 | Corrupt ephemeral key splits DKG group view | `dkg/protocol.go:84-104` (skip-branch), `dkg/marshaling.go` (`unmarshalPublicKeyMap` key-only validation) |
| G4-F9 | Medium | Confirmed | PR #4282 | Corrupt ephemeral key splits signing group view | `signing/protocol.go:84-104` (signing-side skip-branch, byte-identical to G4-F8) |
| G4-F10 | Low | Confirmed | pre-existing gap; PR soft check | Moving-funds follower warns, signs anyway | moving_funds.go:380-389 (ValidateMovingFundsProposal warn-only fee floor) |
| G4-F11 | Low | Confirmed | pre-existing gap; PR soft check | Redemption follower warns, signs anyway | redemption.go:401-413 (ValidateRedemptionProposal warn-only fee floor) |
| G4-F13 | - | False positive | PR #4282 | Package fee vars race at startup | tbtc.go:179-186, 215 (applyWalletTxFeePolicy package vars) |
| G4-F22 | - | Disputed | PR #4282 | setPerformanceMetrics double call leaks tracker | tbtc.go:282 (setPerformanceMetrics call site in Initialize) |

## Confirmed findings

### G4-F6. Reservation flag asymmetry strands anchors with no on-chain proof (Confirmed, Low)

`cmd/start.go:236-243`: when `Tbtc.ReservationsEnabled` is on but the paired
`Maintainer.Spv.ReservationProofsEnabled` (read from the start process's own config file) is off, the start
process only logs a warning and continues: the proposal generator is constructed with reservations enabled
(line 168-172), so the client proposes reservation anchors on-chain while no SPV process submits the proof
loops that would settle them (the proof loop runs only in the maintainer process, gated on its own flag in
spv.Initialize). Effect: anchors proposed but never proven; reservation actions expire or strand until the
timeout/stranding watchers fire.

Mechanism traced end-to-end, hence Confirmed. Severity Low: this is an operator misconfiguration, not a
remote attack; the client-side watchers (wired unconditionally when reservations are enabled) still notify
stranding and action timeouts, so funds recover through the timeout paths rather than being lost. The
start.go comment (lines 218-235) documents that a split deployment's start-process config file has no
reason to contain a `[maintainer.spv]` section, so the paired-flag read is only a hint. The scan's proposed
hard error is wrong for that reason: it would reject valid split deployments. Real fix: escalate on the
client side - the action-timeout watcher already detects pending reservation actions; have it alert (not
just notify on-chain) when a reservation action has been pending without a corresponding SPV proof
submission for N blocks, and ship a deploy-check that compares the two flags across the start/maintainer
config files. Origin: PR-introduced (the whole reservation block, the paired-flag read, and the warn-only
handling are new in the cmd/start.go diff, 99 inserted lines).

Evidence check: drove the real, exported `spv.WireReservationWatchers` (scratch test
`evidence/g4/f6_flag_asymmetry_evidence_test.go`,
`go test -run TestF6_WireReservationWatchers_PairedFlagOff_IsNonFatal ./pkg/maintainer/spv/`) with
`pairedFlagEnabled=false` against an empty chain double (zero wallet registrations, zero reserved deposits,
zero pending actions - the quiet-network shape most likely on a fresh split deployment). Result:
`WireReservationWatchers` returns `nil` (not fatal; the client-side watchers wire up and the process
proceeds), while the pure decision function it defers to, `reservationSelfCheckMisconfigured(false, zero,
zero, zero)`, independently returns `true` for the exact same input and `false` once
`pairedFlagEnabled=true` - confirming `pairedFlagEnabled` is the only signal standing between a quiet fresh
network and a flagged misconfiguration, and that the only consequence of the flagged case is a background
`Errorf` log line, never a returned error or process exit. This directly confirms the "warn-only, never
blocks startup" half of the claim through the exported entry point; the complementary half (the
proof-submission loop `maintainReservationProofs` is started only from `spv.Initialize`, gated on its own
`ReservationProofsEnabled` flag, and is never reached from this call path) remains a code-reading fact
(`pkg/maintainer/spv/spv.go:82`), since reproducing it end to end would require standing up the full
maintainer SPV loop against a chain double, which is out of proportion to what the claim needs. Verdict and
severity unchanged (Confirmed, Low).

### G4-F8. A corrupt ephemeral key splits the DKG group's active view (Confirmed, Medium)

The PR changed the ephemeral key map from `map[MemberIndex]*ephemeral.PublicKey` to
`map[MemberIndex][]byte` (deferred-parse optimization, commits 22388f60d and dfb578f11) and added a
skip-branch in `generateSymmetricKeys` (dkg/protocol.go:84-104, identical in signing/protocol.go): if the
key entry targeted at this member fails to unmarshal, the member marks the sender inactive and continues.
Before the change, one unparseable entry failed unmarshaling of the whole message at the network layer, so
every receiver dropped the entire message and treated the sender as uniformly absent. Now: only the
receiver whose own targeted entry is corrupt marks the sender inactive; every other receiver keeps it. A
malicious in-group member (or one whose key bytes are corrupted in transit) can corrupt exactly one
targeted entry: the group's active/operating sets diverge across members, and a DKG round that would have
completed under the unified view fails, or the shared-key derivation diverges.

Mechanism fully traced: `unmarshalPublicKeyMap` (dkg/marshaling.go) validates only the map keys (member
index range) and accepts arbitrary-length `[]byte` values, so a corrupt entry passes the wire layer and is
only caught at the per-receiver parse. Trigger: an operator inside the DKG group (remote peer, untrusted in
the threat model). Impact: one failed DKG round (recoverable on the next attempt), liveness grief; Medium.
The scan's proposed `dropSenderMessage` does not exist in the codebase. Real fix: validate each ephemeral
key entry's shape (33-byte compressed point) at unmarshal time - that restores the pre-PR unified-drop
behavior at negligible cost - or, if the deferred-parse optimization is kept, broadcast the per-sender parse
failure so all members converge on the same active view. Origin: PR-introduced (the whole branch is new in
the diff).

Second check: `go test -run TestSecondCheck_F8 ./pkg/tecdsa/dkg/...` with a scratch `secondcheck_test.go`
calling the real (unexported) `unmarshalPublicKeyMap` and `ephemeral.UnmarshalPublicKey`. Result:
`unmarshalPublicKeyMap(map[uint32][]byte{2: {0x02}, 3: {0x02}})` returned no error and preserved the 1-byte
value verbatim, confirming the wire layer validates only the member-index key, not the value's shape; a
genuine 33-byte compressed key from `ephemeral.GenerateKeyPair()` parsed via `ephemeral.UnmarshalPublicKey`,
while the same 1-byte garbage value failed to parse - confirming corruption is detectable only at
per-receiver ECDH-parse time (the skip-branch at protocol.go:84-104), not at unmarshal, which is exactly the
mechanism that lets one corrupt entry diverge the group's view per-receiver (only the receiver targeted by
the corrupt entry marks the sender inactive). Building a full two-member divergence through
`symmetricKeyGeneratingMember` was not attempted (would need a heavier membership-validator/group harness);
the proven wire-vs-parse-time split is sufficient to confirm the mechanism as described. Verdict and
severity confirmed unchanged (Confirmed, Medium).

### G4-F9. A corrupt ephemeral key splits the signing group's active view (Confirmed, Medium)

Identical mechanism to G4-F8 in the signing protocol (signing/protocol.go:84-104, same PR commits): a live
tBTC wallet's signing round can have honest members split into different operating sets; a round that should
complete below the dishonest threshold fails, stalling the wallet's signing attempt until the next attempt
re-generates ephemeral keys. Trigger: a malicious in-group operator corrupting one ephemeral key entry.
Impact: signing liveness grief on a live wallet (no funds loss, redeemer/depositor actions stall one
window), Medium. Real fix: as G4-F8.
Origin: PR-introduced.

Second check: `go test -run TestSecondCheck_F9 ./pkg/tecdsa/signing/...` with a scratch test confirming the
signing-side `unmarshalPublicKeyMap` and skip-branch are byte-identical to G4-F8's (same wire-layer /
use-time-parse split). Verdict and severity confirmed unchanged (Confirmed, Medium).

### G4-F10. A follower only warns on a below-floor moving-funds fee and signs anyway (Confirmed, Low)

`ValidateMovingFundsProposal` (moving_funds.go:380-389) ends with the log-only
`warnIfProposedWalletTxFeeBelowBufferedFloor` and then returns nil: a leader that proposes a total fee below
the buffered minimum (as low as the relay floor, since the on-chain `WalletProposalValidator` bounds fees
only from above) is signed by every honest follower. The resulting non-RBF transaction sits in the mempool
until it eventually confirms, jamming the wallet's main UTXO and all actions that need it in the meantime.
Traced end-to-end (the check function never rejects - proposal_fee_check.go:55-108 returns nothing, it only
logs), so Confirmed.

Severity Low and intentional: the code comment in proposal_fee_check.go explicitly documents the log-only
design - rejecting a below-floor proposal during a mixed-version rollout would split signers (patched nodes
reject, unpatched sign) and could stall signing; hard enforcement belongs on-chain or behind a coordinated
all-node upgrade (cites `threshold-network/keep-core#4171`). Trigger: a malicious wallet leader (an operator
in the group; leader rotates per window), griefing only - it can delay the wallet, not steal. The gap
predates the PR; the PR added the intentional log-only check (new file proposal_fee_check.go, 151 lines) as
an interim stopgap. Fixed with the other four fee paths by the hard reject described in P2.

### G4-F11. A follower only warns on a below-floor redemption fee and signs anyway (Confirmed, Low)

Same mechanism and verdict as G4-F10 for redemptions (`ValidateRedemptionProposal`, redemption.go:401-413): a
malicious leader's relay-floor fee proposal is signed by all followers; the redemption tx is delayed in the
mempool and the wallet's main UTXO is held up while unconfirmed. The PR's documented stopgap, Low. Origin:
pre-existing gap; PR added the soft check. Fixed with the other four fee paths by the hard reject described
in P2.

## Disputed and false positive findings

### G4-F7. Failed watcher wiring kills the whole node (Disputed)

The only error returns in `WireReservationWatchers` are two nil-chain guards
(reservation_wiring.go:234-238), which are programming errors, not runtime or operator conditions. Every
runtime failure the function can hit is handled internally with warnings and retries: startup-scan failures
warn and continue (lines 275-284, 297-354), per-wallet fetch/check failures warn after 3 attempts and queue
a second-chance retry (lines 300-354, 389-397), watcher run failures log at Errorf and record a
watcher-death metric instead of returning (lines 455-471, 511-521), and the misconfiguration self-check is
deliberately non-fatal by design (lines 524-566, returning the final `nil` at line 568). The call site's
`logger.Fatal` (cmd/start.go:257-269) is therefore unreachable for any non-malicious operator state.
Disputed: cite the internal handling above; no guard change needed. (If a future edit makes the function
return scan errors, the fatal path becomes real and this should be re-triaged.)

### G4-F13. Package-level fee-policy variables race between startup write and reads (False positive)

`applyWalletTxFeePolicy` writes the two package vars exactly once, at the top of Initialize (tbtc.go:215
calling the function defined at 179-186), which happens-before the coordination goroutine that later reads
them (the goroutines are spawned by `runCoordinationLayer` at tbtc.go:257, well after the write). Per the Go
memory model the spawned goroutine sees the written values; there is no second writer in production (the
test-only overrides cited in the scan's diff are test paths). The reads are therefore synchronized by the
spawn edge, not racy. False positive: no data race exists in the production path; if test overrides grow,
make them go through a small accessor. (A separate, genuine data race on the same tracker pointer write is
acknowledged in the triage notes of this pass and is not part of this record.)

### G4-F22. setPerformanceMetrics called twice leaks the old tracker (Disputed)

There is exactly one call site in production: tbtc.go:282 inside Initialize. The "second call" premise is
unreachable. The claimed leak is also not established: after reassignment the old tracker has no references
left - the goroutines read the `node.windowMetricsTracker` field (the current value), not the old tracker -
so it is garbage-collectible. Disputed: cite the single call site and the rewire semantics (node.go:204-228).
The genuine adjacent defect is the pointer-write/read data race between that call (which runs after the
coordination goroutines are already spawned) and their unlocked reads, which is recorded separately. Origin:
PR-introduced (the tracker plumbing is new in this PR).

## Proof programs used

- The window-map simulation program was deleted after the run; it reproduced the evict-then-read nil-deref
  in the pre-existing coordination window metrics, and both the harness and that path are outside this
  record.
- The bare-goroutine program was deleted after the run: bare goroutine, panicking payload, no recover;
  process exits code 2 with the runtime panic trace, proving that a panic in any of these goroutines kills
  the whole node.

## Second-check re-verification (2026-10-03)

The scratch programs above were never re-run (they were deleted before this pass started); their claims
were independently re-derived instead, using a detached worktree at `49c4df13d` (created outside the repo
and removed after use) with scratch `secondcheck_test.go` files added to the relevant packages, run with
`go test -run ...`, then removed with the worktree. See the per-item "Second check:" lines above for exact
commands and results. Net result: the general "panic in an unrecovered goroutine kills the process"
semantics were independently reproduced with a fresh standalone `go run` scratch (exit code 2); the
ephemeral-key wire-vs-parse-time split (G4-F8/F9) was reproduced directly against the unexported
unmarshaller.
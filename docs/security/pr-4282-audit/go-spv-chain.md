# G2 go-spv-chain triage (PR #4282)

Evidence note: the solidity-auditor scan output named in this file is an internal run record and is not
part of this record. The `evidence/g2/` scratch tests referenced here are in this tree (`evidence/`,
committed with the report).

## Scope
- Scan run: (record not published; stamp `20261001-130005`)
- Files (scope.tsv): `pkg/maintainer/spv/reservation_proof_loop.go`, `reservation_wiring.go`,
  `reservation_action_timeout_watch.go`, `reservation_stale_deposit_watch.go`,
  `reservation_reanchor_proof.go`, `reservation_stranding_watch.go`, `reservation_event_scan.go`,
  `reservation_acceptance_proof.go`, `spv.go`, `chain.go`, `config.go`, and `pkg/chain/ethereum/tbtc.go`.
- On-chain guards cross-checked against `tbtc-v2@eec999aad:solidity/contracts/` (tbtc-v2 `eec999aad`):
  `bridge/Reservation.sol`, `bridge/ReservationProofs.sol`, `bridge/WalletProposalValidator.sol`,
  `bridge/BitcoinTx.sol`.
- x-ray invariant I-11 (watchers compare wall clock to chain time) is the only unenforced invariant; the
  leads L7, L11, L13 and several staleness checks reduce to it. The SPV maintainer is a permissionless
  off-chain bot, so impact is framed as liveness / gas waste / missed proofs, not fund loss.

## Counts
Findings: 24. The triage this comes from also holds a row on the shared pre-existing `getProofInfo`
expression; it is not part of this record - see `Origin` below.
- Confirmed: 9 (F1, F2, F9, F10, F11, F15, F17, F22, F25)
- Plausible: 8 (F3, F4, F5, F8, F16, F18, F19, F24)
- Disputed: 2 (F12, F21)
- False positive: 3 (F13, F14, F23)
- Duplicate: 2 (F7 of F1, F20 of F19)

Severity (Confirmed/Plausible only): High 1 (F2), Medium 1 (F1), Low 15. All are liveness / gas-waste
issues on a permissionless keeper, recoverable by the protocol, by other operators, or by a raw
permissionless notice.

Leads: the per-source lead table is not published; the leads the fix plan references are listed in the
index's appendix.

Origin: F1-F5, F7-F25 are `Introduced by PR #4282` (every reservation file is new in the diff;
`git diff a7ac8989 49c4df13d` shows them as insertions).

Prior knowledge: no finding or lead in this file has an earlier reservation-path report against it; the
reservation loops are new code in this PR.

## Summary table

| ID | Severity | Verdict | Origin | Title | Location |
|---|---|---|---|---|---|
| G2-F1 | Medium | Confirmed | PR #4282 | Abandoned reanchor generations never get evicted | `settleableReanchorReservation` (reservation_proof_loop.go:680) |
| G2-F2 | High | Confirmed | PR #4282 | Zero wallet check blocks the acceptance timeout notice | `checkReservationActionTimeout` (reservation_action_timeout_watch.go:575) |
| G2-F3 | Low | Plausible | PR #4282 | Unbatched per-tick scan over every tracked deposit | `pollTick` (reservation_stale_deposit_watch.go:567) |
| G2-F4 | Low | Plausible | PR #4282 | Wallet-close handler scans registrations from genesis | `resolveWalletPublicKeyHash` (reservation_wiring.go:759) |
| G2-F5 | Low | Plausible | PR #4282 | Transaction limit truncation hides old reanchor proofs | `walletTransactionsForProof` (reservation_proof_loop.go:172) |
| G2-F7 | - | Duplicate of G2-F1 | PR #4282 | Idle reanchor generations grow the event map forever | `proveReservationReanchorActions` (reservation_proof_loop.go:868) |
| G2-F8 | Low | Plausible | PR #4282 | One candidate's read error hides a later confirmed replacement | `proveReservationTransaction` (reservation_proof_loop.go:1129) |
| G2-F9 | Low | Confirmed | PR #4282 | One failed reservation read drops every stranding notice for the wallet | `resolveCustodiedReservationKeys` (pkg/chain/ethereum/tbtc.go:1546) |
| G2-F10 | Low | Confirmed | PR #4282 | Registration lookup failure triggers a genesis-wide scan | `WalletReservations` (pkg/chain/ethereum/tbtc.go:1326) |
| G2-F11 | Low | Confirmed | PR #4282 | Termination-cause lookup falls back to a full scan | `WalletTerminationCause` (pkg/chain/ethereum/tbtc.go:1077) |
| G2-F12 | - | Disputed | PR #4282 | Matcher accepts an anchor below the Bridge's minimum | `isMatchingReservationAcceptanceTransaction` (reservation_proof_loop.go:610) |
| G2-F13 | - | False positive | PR #4282 | Matcher rejects the zero-fee anchor the Bridge allows | `isMatchingReservationAcceptanceTransaction` (reservation_proof_loop.go:617) |
| G2-F14 | - | False positive | PR #4282 | Reanchor matcher rejects the zero-fee reanchor the Bridge allows | `isMatchingReservationReanchorTransaction` (reservation_proof_loop.go:1050) |
| G2-F15 | Low | Confirmed | PR #4282 | Failed submission blocks a sibling generation in the same pass | `proveReservationAcceptanceActions` (reservation_proof_loop.go:563) |
| G2-F16 | Low | Plausible | PR #4282 | Stuck pending acceptance generations grow the event map forever | `proveReservationAcceptanceActions` (reservation_proof_loop.go:421) |
| G2-F17 | Low | Confirmed | PR #4282 | Failed submission blocks a sibling generation in the same pass (reanchor) | `proveReservationReanchorActions` (reservation_proof_loop.go:1000) |
| G2-F18 | Low | Plausible | PR #4282 | A failed reanchor timeout notice skips the stranding re-queue | `checkReservationActionTimeout` (reservation_action_timeout_watch.go:680) |
| G2-F19 | Low | Plausible | PR #4282 | A clean skip drops a timed-out reservation's stranding recheck | `drainStrandingRechecks` (reservation_action_timeout_watch.go:746) |
| G2-F20 | - | Duplicate of G2-F19 | PR #4282 | Drain drops closing wallets instead of retrying them | `drainStrandingRechecks` (reservation_action_timeout_watch.go:746) |
| G2-F21 | - | Disputed | PR #4282 | Historical deposits are matched against today's vault address | `trackRevealedDeposits` (reservation_stale_deposit_watch.go:609) |
| G2-F22 | Low | Confirmed | PR #4282 | A single reservation read failure drops silently | `checkReservationStrandingForWallet` (reservation_stranding_watch.go:149) |
| G2-F23 | - | False positive | PR #4282 | ActionPending reservations skip stranding checks | `checkReservationStrandingForWallet` (reservation_stranding_watch.go:173) |
| G2-F24 | Low | Plausible | PR #4282 | Startup stranding scan never checks a dissolution-eligible Closing wallet | `retryReservationStrandingStartupScan` (reservation_wiring.go:692) |
| G2-F25 | Low | Confirmed | PR #4282 | Unrelated wallet registrations hide a reservation flag mismatch | `reservationSelfCheckMisconfigured` (reservation_wiring.go:599) |

## Confirmed findings

### G2-F2 - Zero wallet check blocks the acceptance timeout notice (High, Confirmed)
The zero-wallet skip at reservation_action_timeout_watch.go:575-579 is not a corner case; it is the normal
case for every first-generation acceptance. The on-chain position is created at reveal with
`walletPubKeyHash` zero; it is only written at settlement (bridge/ReservationProofs.sol:563), and
`requestReservationAcceptance` requires the position to be `Unknown` (bridge/Reservation.sol:521), so for
the whole Pending-acceptance window the position reads zero. `checkReservationActionTimeout` therefore
skips every first-generation timeout notice, so `NotifyReservationAcceptanceTimedOut` is never submitted by
any standard operator for a fresh reservation. Impact path (griefing, permissionless, no fund loss): a
depositor reveals a reserved deposit to a chosen target wallet, requests acceptance (which reserves
capacity of `deposit.amount` against that wallet, bridge/Reservation.sol:610), then never anchors. The
action times out but is never noticed, so the reserved capacity stays pinned (release only happens inside
`notifyReservationAcceptanceTimedOut`, bridge/Reservation.sol:687-689) and the deposit key cannot be
re-requested while the old generation is Pending (bridge/Reservation.sol:525). Repeating cycles exhausts
the target wallet's `maxReservationsPerWallet` / amount caps, breaking its ability to host new
reservations. Likelihood: high in any real usage (every abandoned first generation pins capacity until a
raw permissionless notice or late settlement). Recovery: permissionless
`notifyReservationAcceptanceTimedOut` / governance, so liveness not funds. Fix: when the reservation
position's wallet field is zero, fall back to the action's `TargetWalletPublicKeyHash` (the capacity release
keys on it) instead of skipping, or drop the skip for `Acceptance`-type actions entirely.

Second check: Confirmed. Scratch test `TestG2F2_ZeroWalletPendingAcceptance_SkipsAcceptanceTimeoutNotice`
(zero-wallet reservation with pending Acceptance, now past deadline) -> 0 acceptance-timeout notifications;
control with non-zero wallet -> 1 notification. `ReservationProofs.sol:563` is the only write site for
`reservation.walletPubKeyHash`; `requestReservationAcceptance` writes `action.targetWalletPublicKeyHash`
but not `reservation.walletPubKeyHash`. `notifyReservationAcceptanceTimedOut` (ReservationRouter:323) is
`external` with no `onlyGovernance` gate: permissionless. `NotifyReservationAcceptanceTimedOut` is called
only by the SPV maintainer timeout watcher (grep confirmed); no other Go path sends it. Standard operator
notice path is fully dead for every first-generation pending acceptance. Severity High held: operator
notice path dead for all fresh acceptances; permissionless recovery exists on-chain but no standard
operator will run it; unbounded capacity-pin griefing follows.

### G2-F1 - Abandoned reanchor generations never get evicted (Medium, Confirmed)
`settleableReanchorReservation` (reservation_proof_loop.go:680-719) keeps a TimedOut reanchor generation
tracked while the reservation is Active / ActionPending / Stranded and the source anchor still matches the
generation's snapshot; it is evicted only when the reservation leaves those states or the anchor moves past
the snapshot. The comment at reservation_proof_loop.go:678 states this is deliberate: "Re-anchor late
settlement is otherwise unbounded, so age alone never triggers an eviction." Growth is
permissionless-amplified: `requestReservationReanchor` (bridge/Reservation.sol:723-859) has no caller
restriction for the non-privileged path, so anyone can cycle request -> timeout (the cooldown at line 739
rate-limits but does not bound the total) against a MovingFunds/Closing source wallet. Each timed-out
generation whose anchor never moves (a dead source wallet, or no proof ever settles) stays in
`pendingReanchorEvents` plus a `reanchorBackoff` entry for the process lifetime. The 32-pass backoff
(reservation_proof_loop.go:746-791) bounds the read cadence, not the count, so memory and periodic RPC
reads grow without limit. Impact: operator DoS (memory + RPC), no fund loss. Fix: evict a tracked
generation once it can no longer settle - i.e. bound it to the on-chain late-settlement horizon (a
configurable max generation age, or evict when a newer generation supersedes and the older one's proof
window has closed on-chain).

Second check: Confirmed. `TestProveReservationReanchorActions_TimedOutSourceAnchor/unchanged_source_anchor_is_kept_regardless_of_age`
confirms TimedOut generation stays in `pendingReanchorEvents` across 3 passes at now=4_000_000_000 (far
future); evicted only when source anchor is replaced. `TestProveReservationReanchorActions_TimedOutUnbounded`
confirms no bounded late window: timed-out generation remains tracked and proved indefinitely.
`settleableReanchorReservation` (reservation_proof_loop.go:680): retain while reservation in
Active/ActionPending/Stranded AND source anchor unchanged. On-chain `requestReservationReanchor` is
permissionless (no caller gate for non-privileged path); cooldown (line 739) rate-limits but does not bound
total cycles. Each request-then-notify-timeout cycle adds one TimedOut generation that stays forever until
anchor moves; unbounded growth in `pendingReanchorEvents` per wallet. Severity Medium held.

### G2-F9 - One failed reservation read drops every stranding notice for the wallet (Low, Confirmed)
`resolveCustodiedReservationKeys` (pkg/chain/ethereum/tbtc.go:1546-1554) returns on the first `GetReservation`
error, so `WalletReservations` (pkg/chain/ethereum/tbtc.go:1359) aborts the whole lookup. In
`checkReservationStrandingForWallet` (reservation_stranding_watch.go:97) that aborts the walk, so every
other reservation on the same wallet misses its stranding notice for this triggering event. Impact: delayed
`notifyReservationStranded` for the other reservations (liveness, recoverable by another operator or a later
event). Fix: log and `continue` past a per-key read error instead of `return` so one bad key does not
starve the rest.

Evidence check: scratch test `TestG2F9_ResolveCustodiedReservationKeysAbortsOnFirstLookupError`
(`go test -run TestG2F9 ./pkg/chain/ethereum/...`, evidence/g2/tbtc_f9_evidence_test.go) seeds two
candidate keys for one wallet where key 1's lookup fails and key 3's would succeed; result: `queried=[1]` -
key 3 is never looked up and `resolveCustodiedReservationKeys` returns no keys at all, confirming the abort
drops every other reservation on the wallet, not just the failing key. This extends the package's existing
`TestResolveCustodiedReservationKeys/lookup_error_short-circuits_the_whole_call` subtest, which only showed
an error is returned. The `tc.GetReservation`-backed production call needs a live/simulated Ethereum node
that pkg/chain/ethereum has no test harness for (package-wide gap); only the injected pure function is
executed here.

### G2-F10 / G2-F11 - Registration lookup failure triggers a genesis-wide scan (Low, Confirmed)
`earliestWalletRegistrationBlock` (pkg/chain/ethereum/tbtc.go:1383) fails on a transient RPC error
(current-block read or the doubling registration walk, lines 1390-1416). `WalletReservations` (line
1326-1329) and `WalletTerminationCause` (line 1077-1080) both set `startBlock = 0` on that error, degrading
to a chunked full-history scan (10,000-block chunks, `fetchPastEventsInChunks`); on mainnet that is hundreds
to thousands of `eth_getLogs` calls that most providers reject. The fallback is documented intent
(pkg/chain/ethereum/tbtc.go:1320-1325: "degrades to the previous behavior rather than skipping real
reservations"), so the cost is a known trade-off: heavy RPC/gas and a probable provider rejection, which
then fails the stranding check. Impact: operator RPC load / availability, no fund loss. Fix: propagate the
lookup error instead of silently widening to genesis, and rely on the per-call retry in the stranding
watcher.

Evidence check: code reading only for the TbtcChain-level fallback itself - `tc.earliestWalletRegistrationBlock`
needs `tc.blockCounter` and `tc.bridge` (generated bindings backed by a real/simulated Ethereum client, a
package-wide test-harness gap in pkg/chain/ethereum). The pure-logic core it depends on is executed:
`go test -run TestEarliestRegistrationEventBlock ./pkg/chain/ethereum/...` passes (existing test, 4
subtests), proving `resolveEarliestWalletRegistrationBlock`'s error propagation. The
`if err != nil { startBlock = 0 }` fallback immediately above both call sites (tbtc.go:1326-1329,
1077-1080) is a direct read, not an inference.

### G2-F15 / G2-F17 - A failed submission blocks a sibling generation in the same pass (Low, Confirmed)
`proveReservationTransaction` returns `(reservationProofSubmitted, err)` when the submit itself failed
(reservation_proof_loop.go:1134-1136). Both proof loops mark the reservation key as submitted on that
outcome (`if outcome == reservationProofSubmitted { submittedReservations[reservationKey] = ... }`,
reservation_proof_loop.go:563-565 and 1000-1002) before checking `err`, so a later, independently
settleable generation of the same key is skipped for the rest of the pass (reservation_proof_loop.go:502,
933). Per-key dedup is intentional (x-ray G-52); the bug is that a failed submit still claims the key.
Impact: one-pass liveness delay, self-heals on the next pass. Fix: mark
`submittedReservations[reservationKey]` only when `err == nil`.

Evidence check: scratch tests `TestG2F15_FailedSubmissionBlocksSiblingGeneration` and
`TestG2F17_FailedReanchorSubmissionBlocksSiblingGeneration`
(`go test -run 'TestG2F15|TestG2F17' ./pkg/maintainer/spv/...`): each seeds a Pending and a TimedOut
generation of the same reservation key with a submit hook that fails on the first call; result in both:
exactly one submission attempt, at the Pending generation's nonce (sorted first) - the TimedOut sibling is
never attempted in that pass, confirming the failed submit claims the key.

### G2-F22 - A single reservation read failure drops silently (Low, Confirmed)
`checkReservationStrandingForWallet` (reservation_stranding_watch.go:149-167) wraps `GetReservation` in
`withStrandingRetry` (3 attempts); after the attempts it logs and `continue`s, so that reservation's
stranding is never re-attempted for this triggering event. Impact: one reservation's stranding notice is
missed for this event (liveness; the Bridge still permits a late `notifyReservationStranded`, and the
startup scan or another operator can recover). Fix: requeue the wallet for a later stranding pass on a
failed read rather than silently continuing.

Evidence check: scratch test `TestG2F22_SingleReservationReadFailureDropsOnlyThatKey`
(`go test -run TestG2F22 ./pkg/maintainer/spv/...`): one wallet with two reservation keys, `GetReservation`
failing for one key only (retries exhausted); result: `checkReservationStrandingForWallet` returns nil (the
drop is silent to the caller), the healthy key is notified, the failing key is not.

### G2-F25 - Unrelated wallet registrations hide a reservation flag mismatch (Low, Confirmed)
`reservationSelfCheckMisconfigured` (reservation_wiring.go:599-601) requires `registration.count == 0` in
addition to the two reservation scans being zero. The registration scan covers the last 30 days of wallet
registrations (reservation_wiring.go:67-72, 609-653), which is non-zero on any active network, so the
misconfiguration Errorf (reservation_wiring.go:553-565) never fires there even when
`Tbtc.ReservationsEnabled` and `Maintainer.Spv.ReservationProofsEnabled` are genuinely misconfigured.
Impact: a missed operator-facing log; the watchers still run, so no missed notifications - a monitoring gap
only. Fix: drop the `registration.count == 0` term so the self-check keys only on the reservation-specific
scans.

Evidence check: scratch test `TestG2F25_SelfCheckKeysOnRegistrationCount`
(`go test -run TestG2F25 ./pkg/maintainer/spv/...`), driving the pure `reservationSelfCheckMisconfigured`
function directly: all-zero scans -> signal fires; non-zero registration count with both reservation scans
still zero -> signal does not fire, confirming the registration count masks a genuine misconfiguration.

## Plausible findings

### G2-F3 - Unbatched per-tick scan over every tracked deposit (Low, Plausible)
`pollTick` (reservation_stale_deposit_watch.go:567-594) iterates every tracked deposit each minute. Before a
refund deadline each costs no chain reads (reservation_stale_deposit_watch.go:262); after the deadline each
costs reads on every tick until the record clears on-chain. An attacker revealing many reserved deposits
grows per-tick RPC volume. Unverified precondition: the attacker's reveal volume is bounded only by
`pendingReservedDeposits` (there is no count cap - the contract comment on `forceStaleReservedDeposit`,
bridge/Reservation.sol:1171-1178, notes malicious depositors can "grind" it), but each deposit requires real
Bitcoin funding, capping the grief. Trigger: a malicious high-volume depositor. Impact: operator RPC load,
no fund loss. Fix: cap per-tick checks with a rotating cursor like `pollPendingActions` already does
(reservation_action_timeout_watch.go:35-35, 462-501).

Evidence check: scratch test `TestG2F3_StaleDepositPollTickReadsScaleWithTrackedCount`
(`go test -run TestG2F3 ./pkg/maintainer/spv/...`): 3 past-deadline reserved deposits are tracked and
notified on tick 1; tick 2 re-reads all 3 (`ReservedDepositWallet` call count 3 -> 6, measured via a
counting chain wrapper) with no per-tick cap - reads scale linearly with the tracked count, as claimed. The
attacker reveal-volume precondition remains unverified (an on-chain precondition, not a Go code path).

### G2-F4 - Wallet-close handler scans registrations from genesis (Low, Plausible)
`resolveWalletPublicKeyHash` (reservation_wiring.go:759-767) builds a `NewWalletRegisteredEventFilter` with
only `EcdsaWalletID` and no `StartBlock`, so it scans from genesis. Most RPC providers cap `eth_getLogs`
ranges and reject it; on failure the one-shot close handler logs and returns
(reservation_wiring.go:730-737), so that close's stranding check is skipped. Unverified precondition: a
provider that rejects the unbounded range. Impact: missed stranding notice for that close event (liveness,
recoverable via the startup scan / other operators). Fix: bound the filter with the wallet's earliest
registration block (reuse `earliestWalletRegistrationBlock`, pkg/chain/ethereum/tbtc.go:1383).

Evidence check: scratch test `TestG2F4_CloseHandlerResolutionFailureDropsTheCheck`
(`go test -run TestG2F4 ./pkg/maintainer/spv/...`), two subtests: (1) the close handler's
registration-lookup filter carries `StartBlock 0` (recorded via a wrapper chain) - confirmed unbounded;
(2) injecting a lookup error drops the stranding check entirely (0 notifications) even though the wallet's
reservation would otherwise strand - confirmed the close event's check is skipped, not retried.

### G2-F5 - Transaction limit truncation hides old reanchor proofs (Low, Plausible)
`walletTransactionsForProof` (reservation_proof_loop.go:172-174) keeps only the newest
`config.TransactionLimit` (default 20, pkg/maintainer/spv/config.go:25). A TimedOut reanchor generation whose
reanchor transaction is not among the wallet's newest 20 never reappears as a candidate, so this operator
never submits its late proof. Unverified precondition: a wallet that broadcast more than 20 transactions
after its reanchor transaction (a MovingFunds/Closing wallet doing sweeps) - uncommon, so likelihood is low.
Impact: a still-settleable late reanchor proof is missed by this operator (liveness; the Bridge settles it
unbounded, so any operator or a future window can still prove it). Fix: for TimedOut reanchor generations,
fetch by the expected outpoint instead of the newest-20 window.

Evidence check: scratch test `TestG2F5_TransactionLimitTruncatesOldReanchorProof`
(`go test -run TestG2F5 ./pkg/maintainer/spv/...`): a TimedOut reanchor generation with 25 filler
transactions broadcast after it; at the default limit (20) the reanchor transaction is truncated out of the
wallet's newest-20 window and 0 proofs are submitted (generation stays tracked); the same fixture against a
fresh scan state with limit 45 finds the same transaction and submits 1 proof - confirming the proof was
reachable all along and only hidden by the limit.

### G2-F8 - One candidate's read error hides a later confirmed replacement (Low, Plausible)
`proveReservationTransaction` (reservation_proof_loop.go:1121-1131) returns `reservationProofNoMatch` on the
first `getProofInfo` error, so later confirmed RBF replacements in the candidate list are not examined this
pass. Unverified precondition: a transient chain-read failure on an earlier candidate (per-tick Electrum
blip) while a wallet broadcast multiple replacements for the same outpoint. Impact: a provable replacement
is not submitted this pass (liveness, self-heals next pass). Fix: `continue` past a per-candidate
`getProofInfo` error so the remaining candidates are still considered.

Evidence check: scratch test `TestG2F8_EarlierCandidateConfirmationsErrorHidesLaterReplacement`
(`go test -run TestG2F8 ./pkg/maintainer/spv/...`): a healthy control (2-confirmation pending candidate
first, 20-confirmation confirmed replacement second) submits the replacement normally (1 submission); the
same two candidates against a chain that fails only the first candidate's confirmations read returns an
error with 0 submissions - the confirmed replacement is never examined that pass.

### G2-F16 - Stuck pending acceptance generations grow the event map forever (Low, Plausible)
`proveReservationAcceptanceActions` (reservation_proof_loop.go:421-477) keeps a Pending acceptance
generation in `pendingAcceptanceEvents` until its state leaves Pending (or, if TimedOut, until
`timeoutAt + termSeconds` at line 445). If the action-timeout watcher never times the generation out (dead
watcher, or a capacity-pinned first generation as in G2-F2), the entry is re-read every pass. Unverified
precondition: a generation that stays Pending for a long time. Impact: bounded-per-key memory/RPC reads that
persist for the process life (liveness cost, no fund loss). Fix: evict a Pending generation after a
configurable age, or gate tracking on the action-timeout watcher's health.

Evidence check: scratch test `TestG2F16_StuckPendingAcceptanceGenerationTracksForever`
(`go test -run TestG2F16 ./pkg/maintainer/spv/...`): a Pending acceptance generation driven through 5 passes
at `now = 4_000_000_000` (far future) stays tracked and is re-read via `GetReservationAction` exactly once
per pass (5 reads total, measured via a counting chain wrapper) - no age bound evicts it.

### G2-F18 - A failed reanchor timeout notice skips the stranding re-queue (Low, Plausible)
`checkReservationActionTimeout` (reservation_action_timeout_watch.go:680-682) enqueues the wallet for the
deferred stranding recheck only after `NotifyReservationActionTimeout` succeeds. If the notice reverts (the
action left Pending between read and send) or the RPC fails persistently, the recheck is never enqueued, so a
dead wallet's reservation is not rechecked for stranding by this operator on that path. Unverified
precondition: a notice that does not land. Impact: missed stranding recheck until another path runs
(liveness; `notifyReservationStranded` remains permissionless and the late-settlement path strands
automatically, bridge/ReservationProofs.sol:228-249). Fix: enqueue `strandingRecheckWallets` regardless of
the notice outcome.

Evidence check: scratch test `TestG2F18_FailedReanchorTimeoutNoticeSkipsStrandingRequeue`
(`go test -run TestG2F18 ./pkg/maintainer/spv/...`): `checkReservationActionTimeout` with a failing
`NotifyReservationActionTimeout` returns an error and leaves `strandingRecheckWallets` empty - confirmed the
recheck is enqueued only on a successful notice.

### G2-F19 - A clean skip drops a timed-out reservation's stranding recheck (Low, Plausible)
`drainStrandingRechecks` (reservation_action_timeout_watch.go:718-765) requeues a wallet only on error. A
wallet still in the Closing state (not Closed/Terminated, line 746-748) is skipped with `continue` and not
requeued, and a clean check on an ActionPending reservation (the stranding check's own skip at
reservation_stranding_watch.go:173-181) is dropped the same way. Unverified precondition: a Closing wallet
with an unmined timed-out reanchor, or an ActionPending reservation past its dissolution deadline. Impact:
the reservation's stranding notice is not reissued by this operator for that drain (liveness, recoverable
via settlement auto-stranding and other operators). Fix: requeue the wallet on the non-terminal (Closing /
still-pending) branch.

Evidence check: scratch test `TestG2F19_DrainDropsClosingWalletAndCleanPendingSkip`
(`go test -run TestG2F19 ./pkg/maintainer/spv/...`), two subtests: a still-Closing wallet is dropped from
`strandingRecheckWallets` with no stranding notification; a Closed wallet whose only reservation is
ActionPending is dropped the same way (clean skip, no requeue) - both confirmed, no notification issued in
either case.

### G2-F24 - Startup stranding scan never checks a dissolution-eligible Closing wallet (Low, Plausible)
Both the startup scan (reservation_wiring.go:324-327) and the retry pass (reservation_wiring.go:692-693)
forward only Closed / Terminated wallets to the stranding check; a Closing wallet past its
`dissolutionEligibleAt` is skipped even though the Bridge would strand it (bridge/Reservation.sol:1106-1113
accepts Closing once `block.timestamp >= dissolutionEligibleAt`). Unverified precondition: the maintainer was
down when the wallet closed, and it is Closing-eligible rather than Closed/Terminated at startup. Impact: its
stranding notices are missed by this operator's startup scan (liveness, the live `OnWalletClosed` handler and
other operators recover). Fix: include dissolution-eligible Closing wallets in the forwarded set.

Evidence check: scratch test `TestG2F24_StartupStrandingScanNeverChecksClosingWallet`
(`go test -run TestG2F24 ./pkg/maintainer/spv/...`): `retryReservationStrandingStartupScan` run once per
wallet state (Closed, Terminated, Closing, Live), each with one Active reservation that would strand if
checked; result: only Closed and Terminated wallets are notified, Closing and Live are not - confirming the
dissolution-eligible Closing wallet is skipped.

## Disputed findings

### G2-F12 - Matcher accepts an anchor below the Bridge's minimum (Disputed)
`isMatchingReservationAcceptanceTransaction` (reservation_proof_loop.go:610-625) checks the fee bound but not
`anchor >= minAmount`. The exploit is stopped on-chain: `validateAnchorOutput` reverts with "Anchor below
reservation minimum" (bridge/ReservationProofs.sol:463-466), and a legitimate anchor transaction cannot
produce a below-minimum anchor in the first place - the proposal validator enforces
`depositRequest.amount >= action.minAmount + anchorTxFee` (bridge/WalletProposalValidator.sol:1032-1034) and
the Go marshaling guards G-3/G-17 require a positive fee bounded by `TxMaxFee`. So no authorized signer
broadcasts a transaction the matcher would accept but the Bridge would reject.

### G2-F21 - Historical deposits are matched against today's vault address (Disputed)
`trackRevealedDeposits` (reservation_stale_deposit_watch.go:609) filters reveal events against the
`reservationVault` read at scan time. The claimed exploit is stopped on-chain: `updateReservationParameters`
requires `pendingReservedDeposits == 0` before the `reservationVault` may change
(bridge/Reservation.sol:1313), so no reserved deposit is pending while the vault address changes; a deposit
custodied by a wallet cannot observe the vault moving out from under it while it is still reserved. The
vault read at scan time is therefore safe for every tracked deposit.

## False-positive findings

### G2-F13 - Matcher rejects the zero-fee anchor the Bridge allows (False positive)
The Bridge does not allow a zero-fee anchor transaction to exist: `validateReservationAnchorProposal`
requires `anchorTxFee > 0` (bridge/WalletProposalValidator.sol:1037) and the Go marshaling guard G-17 enforces
the same, so no zero-fee anchor is ever signed. The matcher rejecting `fee <= 0`
(reservation_proof_loop.go:617) is correct, not a missed proof. The report's claim that the Bridge "accepts" a
zero-fee anchor is a wrong reading of the code.

### G2-F14 - Reanchor matcher rejects the zero-fee reanchor the Bridge allows (False positive)
`validateReservationReanchorProposal` requires `reanchorTxFee > 0` (bridge/WalletProposalValidator.sol:1190)
and the Go marshaling guard G-20 enforces the same, so a zero-fee reanchor transaction cannot be produced and
the matcher's `fee > 0` check (reservation_proof_loop.go:1051) is correct. The Bridge's separate proof-time
dust-floor check (`newAnchorAmount > action.txMaxFee`, bridge/ReservationProofs.sol:696) is the real
observation behind lead L4, not this finding.

### G2-F23 - ActionPending reservations skip stranding checks (False positive)
The Go skip of a non-Active reservation (reservation_stranding_watch.go:173-181) matches the on-chain guard:
`notifyReservationStranded` reverts unless the reservation is `Active` ("Reservation is not active",
bridge/Reservation.sol:1098-1101). The Bridge itself strands a position during settlement when a late proof
arrives and the wallet is Closing/Closed/Terminated (`strandLateSettlementIfTargetWalletClosed`,
bridge/ReservationProofs.sol:228-249, called at lines 595 and 746). There is no stranding re-notification to
issue for a non-Active reservation; the guard is correct.

## Duplicate findings
- **G2-F7** is a duplicate of **G2-F1**: both describe the same unbounded retention of abandoned reanchor
  generations in `pendingReanchorEvents` / `reanchorBackoff`; F7 frames it from the "map grows" side and F1
  from the "never evicted" side.
- **G2-F20** is a duplicate of **G2-F19**: both are the same `drainStrandingRechecks`
  non-requeue-on-Closing-branch issue, described from two angles.

## Note on severity framing
Every finding here is a liveness / gas-waste / missed-proof issue on a permissionless off-chain keeper. None
are fund loss: the Bridge's on-chain guards (positive-fee proposal validation at
bridge/WalletProposalValidator.sol:1037/1190, the anchor-minimum and dust-floor proof checks at
bridge/ReservationProofs.sol:463/696, the `notifyReservationStranded` preconditions at
bridge/Reservation.sol:1098-1113, the vault-change gate at bridge/Reservation.sol:1313, and the automatic
late-settlement stranding) stop any exploit from reaching a wrong state. The recurring root cause behind the
clock-skew leads (L7, L11, L13) is x-ray invariant I-11 - the only unenforced invariant - where operators
compare wall clock to a `block.timestamp`-gated on-chain deadline. G2-F2 is the notable exception in framing:
it is a permissionless griefing vector (any depositor can pin capacity against a chosen wallet) rather than an
operator-misconfiguration one.

## ID mapping

| Old ID | New ID |
|---|---|
| G2F1 | G2-F1 |
| G2F2 | G2-F2 |
| G2F3 | G2-F3 |
| G2F4 | G2-F4 |
| G2F5 | G2-F5 |
| G2F7 | G2-F7 |
| G2F8 | G2-F8 |
| G2F9 | G2-F9 |
| G2F10 | G2-F10 |
| G2F11 | G2-F11 |
| G2F12 | G2-F12 |
| G2F13 | G2-F13 |
| G2F14 | G2-F14 |
| G2F15 | G2-F15 |
| G2F16 | G2-F16 |
| G2F17 | G2-F17 |
| G2F18 | G2-F18 |
| G2F19 | G2-F19 |
| G2F20 | G2-F20 |
| G2F21 | G2-F21 |
| G2F22 | G2-F22 |
| G2F23 | G2-F23 |
| G2F24 | G2-F24 |
| G2F25 | G2-F25 |
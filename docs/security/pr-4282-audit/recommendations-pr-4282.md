# PR #4282 (UTXO reservations epic) - fix recommendations

Scope: every PR-introduced Confirmed/Plausible finding in the audit goes into PR #4282 before it leaves
draft (human decision D1). Audited code: `49c4df13d` (epic head; merge-base `a7ac8989`; PR diff is
`git diff a7ac8989 49c4df13d`). Finding detail lives in the sibling triage files; executed evidence under
`docs/security/pr-4282-audit/evidence/` (Go files renamed `*.go.txt`).
Counts: 36 findings (High 1, Medium 10, Low 25; Confirmed 22, Plausible 14). All 36 are fixed in the PR
(human decision D1): the five wallet-fee paths (G1-F6, G1-F7, G1-F8, G4-F10, G4-F11) hard-reject below the
fixed protocol minimum, with no activation switch (the switch proposed earlier was dropped, since every
honest leader emits at least the bare floor and the protocol minimum is safe in mixed-version traffic).

Source files: [README.md](./README.md), [go-signing.md](./go-signing.md),
[go-spv-chain.md](./go-spv-chain.md), [go-node-protocol.md](./go-node-protocol.md),
[go-periphery.md](./go-periphery.md).

## Fix order

Packages are grouped by root cause, ordered by severity then blast radius. Each package lists its findings,
exact locations, the concrete change, the regression test to add (naming the existing evidence or
second-check test to convert where one exists), and the observable acceptance check.

### P1. Acceptance timeout notice path (High; G2-F2, G2-F18, G2-F19; lead G2-L9)

- IDs: G2-F2 (High, Confirmed), G2-F18 (Low, Plausible), G2-F19 (Low, Plausible), G2-L9 (lead).
- Location: `pkg/maintainer/spv/reservation_action_timeout_watch.go`, `checkReservationActionTimeout`
  (:555-698: zero-wallet skip at :575-579, stranding re-queue at :680-682, preload reuse at :583-594) and
  `drainStrandingRechecks` (:718-765, non-terminal skip at :746-748).
- Change:
  - G2-F2: remove the zero-wallet early return in `checkReservationActionTimeout`.
    `NotifyReservationAcceptanceTimedOut(reservationKey)` needs only the reservation key, so
    first-generation acceptances (wallet hash is zero until settlement at `ReservationProofs.sol:563`) get
    their timeout notice. Keep the wallet hash only for the reanchor branch's stranding re-queue: when the
    hash is zero there, skip the re-queue but still send the notice.
  - G2-F18: in the reanchor branch, enqueue `strandingRecheckWallets[walletPublicKeyHash]` regardless of
    whether `NotifyReservationActionTimeout` succeeds.
  - G2-F19: in `drainStrandingRechecks`, requeue the wallet on the non-terminal branch
    (`wallet.State != Closed/Terminated` instead of plain `continue`), and on a clean stranding check of a
    still-pending reservation.
  - G2-L9: on the preload nonce-match reuse path (:583-594), re-read a fresh action state before sending the
    notice.
- Regression test: convert `TestG2F2_ZeroWalletPendingAcceptance_SkipsAcceptanceTimeoutNotice` (evidence/g2,
  currently 0 notices vs 1 control) to expect 1 acceptance-timeout notice for the zero-wallet pending
  first-generation. Invert `TestG2F18_FailedReanchorTimeoutNoticeSkipsStrandingRequeue` (a failing notice
  still leaves the wallet in `strandingRecheckWallets`) and
  `TestG2F19_DrainDropsClosingWalletAndCleanPendingSkip` (a Closing wallet and a clean ActionPending skip
  are requeued). Extend `TestReservationActionTimeoutWatcher_StalePreloadNonceMismatchFallsBackToFreshRead`
  with the nonce-match-but-stale case.
- Acceptance check: a fresh reservation with a pending first-generation acceptance past its deadline emits
  `NotifyReservationAcceptanceTimedOut` with a zero wallet; a failed reanchor notice still enqueues the
  stranding recheck; no wallet is silently dropped from the drain.

### P2. Fixed protocol fee minimum (Medium; G1-F6, G1-F7, G1-F8; Low; G4-F10, G4-F11, G1-S3; lead G1-L26)

- IDs: G1-F6 (Medium, Confirmed), G1-F7 (Medium, Confirmed), G1-F8 (Medium, Confirmed), G4-F10 (Low,
  Confirmed), G4-F11 (Low, Confirmed), G1-S3 (Low, Confirmed, new), G1-L26 (lead).
- Locations (one per path): reservation paths - `pkg/tbtc/reservation.go`, `reservationAnchorAction.execute`
  (:497-637, warn call at :579-599) and `reservationReanchorAction.execute` (:692-777, warn call at :719-739).
  Legacy paths - `pkg/tbtc/deposit_sweep.go` `depositSweepAction.execute` (below-floor warn plus sweep metric
  at :258-265 via `checkSweepFeeFloor`), `pkg/tbtc/moving_funds.go` `ValidateMovingFundsProposal` (:380-389),
  `pkg/tbtc/redemption.go` `ValidateRedemptionProposal` (:401-413). All five warn calls live in
  `pkg/tbtc/proposal_fee_check.go`, which is new in this PR.
- Change (human decision D8):
  - Add the protocol constant `ProtocolMinWalletTxSatPerVByteFee = 5`. It equals the pre-PR hard-coded leader
    floor (`minWalletTxSatPerVByteFee = 5` at `pkg/tbtcpg/fee.go:37`, identical formula at merge base) and
    the PR default of `tbtc.walletTxSatPerVByteFloor`. Both pre-PR and PR leaders emit at least 5 sat/vB, so
    the gate is mixed-version safe.
  - Startup validation: reject a `tbtc.walletTxSatPerVByteFloor` set below
    `ProtocolMinWalletTxSatPerVByteFee` (0 = default stays allowed). The floor and buffer are per-operator
    flags (`cmd/flags.go:315-330`, applied at `pkg/tbtc/tbtc.go:181-184`), so a flag below the protocol
    minimum must not lower the group's guarantee.
  - Follower reject on all five paths, replacing every `warnIfProposedWalletTxFeeBelowBufferedFloor` call:
    return an error when `proposedFee < ProtocolMinWalletTxSatPerVByteFee * (followerVsize - 1)`. The
    one-vbyte allowance covers size-estimate differences between leader and follower versions. Why not the
    buffered floor: with defaults (floor 5, buffer 25) an honest leader emits 5 sat/vB for mempool estimates
    of 4 sat/vB or less, while the follower buffered floor is 7 sat/vB, so a buffered-floor reject would
    reject every honest quiet-mempool proposal (executed table: `evidence/g1/fee_floor_leader_vs_follower_test.go`
    and its `.out`, rows `floor=5 buf=25 est=1..4` all `belowBuffered=true`, `belowBare=false`;
    `floor=10 buf=25 est=1..9` the same).
  - Keep the warning, moved to the bare floor: warn when `proposedFee < MinWalletTxSatPerVByteFee *
    followerVsize` (the operator's own floor, no buffer). This removes the G1-S3 false alarm: no honest
    leader output is below the bare floor, so the warning fires only on genuinely degraded fees.
  - G1-S3 (new, fixed in this package): the follower fee-floor warning fires on every honest low-fee
    proposal; covered by the bare-floor warning above.
  - G1-L26 (the once-per-pass floor read going stale in a narrow governance-change window) is a lead: re-read
    the floor at send time if adopted, monitor otherwise.
- Regression tests:
  - Per path (all five), two tests: fee = `ProtocolMin*(vsize-1)-1` is rejected with no signing; fee =
    `ProtocolMin*vsize` signs normally. Deposit sweep (G1-F6), anchor (G1-F7), re-anchor (G1-F8), moving funds
    (G4-F10), redemption (G4-F11).
  - Cross-check table test: feed `applyWalletTxFeeFloor` outputs for estimates 1..10 sat/vB, with default
    flags (floor 5, buffer 25) and non-default flags (floor 5 buffer 0, floor 10 buffer 25, floor 2 buffer 50),
    into the follower reject check and assert none is rejected. Converts the throwaway
    `evidence/g1/fee_floor_leader_vs_follower_test.go` table into a permanent test.
  - Startup test: `tbtc.walletTxSatPerVByteFloor` below the protocol minimum fails startup; 0 (default) and
    values at or above the minimum start.
- Acceptance check: a follower never co-signs a fee below 5 sat/vB minus the one-vbyte allowance on any of the
  five paths; every honest leader fee output passes; the warning fires only below the operator's bare floor; a
  flag below the protocol minimum is refused at startup.

### P3. Re-anchor custody gate (Medium; G1-F2, G1-F12; leads G1-L21, G1-L24, G1-L25, G1-L27)

- IDs: G1-F2 (Medium, Plausible), G1-F12 (Medium, Plausible).
- Locations:
  - Leader side: `pkg/tbtcpg/reservation_reanchor.go`, `ReservationReanchorTask.Run` resume path (:302-327,
    `ProposeReservationReanchor` with `action.TargetWalletPublicKeyHash` at :308) and
    `reanchorTargetSearch.pick` (:950-1002, first Live wallet with capacity wins at :983-998).
  - Follower side (the real control): `pkg/tbtc/reservation.go`, `reservationReanchorAction.execute`
    (:692-777). Today it only checks that the on-chain action is a pending re-anchor and that its target
    matches the proposal (:704-709). It then calls `ValidateReservationReanchorProposal` (:711), and the
    on-chain validator only requires the target to be Live. A malicious leader skips any leader-side check, so
    the follower check is the one that stops G1-F2.
- Change (human decision D9): on-chain age gate only, with no allowlist and no operator configuration.
  - Shared rule `isTargetWalletAged(target [20]byte, headTime time.Time) bool`:
    `GetWallet(target).CreatedAt` (`WalletChainData.CreatedAt`, `pkg/tbtc/chain.go:413`) + the Bridge
    `walletCreationPeriod` (`tbtc.WalletParameters.CreationPeriod`, `pkg/tbtc/parameters.go:25`) <=
    `headTime`.
  - `headTime` is the chain head block timestamp, never `time.Now()` (same root cause as G1-S1/S2). Read it
    with the bounded 30 s `HeaderByNumber` pattern from P8.
  - Follower (required): in `reservationReanchorAction.execute`, right after the target match at :707-709,
    return an error ("re-anchor target wallet younger than the wallet creation period") when the target is not
    aged. `tbtc.Chain` (`pkg/tbtc/chain.go`) needs `GetWalletParameters()`. The Ethereum `TbtcChain` already
    implements it (`pkg/chain/ethereum/tbtc_wallet.go:176`); today only `tbtcpg.Chain` declares it
    (`pkg/tbtcpg/chain.go:28`). It also needs a bounded head-timestamp read. The follower allows a tolerance
    of 2 blocks (24 s) at the boundary, i.e. it accepts when CreatedAt + CreationPeriod <= headTime + 24 s, so
    that a leader and follower one or two blocks apart do not disagree at the cutoff.
  - Leader (avoids wasted proposals): the resume path skips the resume (log and continue) when the on-chain
    target is not aged, with no tolerance. `pick` excludes candidates that are not aged
    (`s.excluded[target] = true`). The leader's check is strict and the follower's is loose, so an honest
    leader never proposes a target that a follower rejects.
  - No allowlist: per-operator allowlists split a signing group, and there is no on-chain list to agree on.
- Fails closed: if the chain head read or the `GetWallet`/`WalletParameters` reads fail, the leader skips the
  target and retries on the next pass, and the follower returns an error instead of signing. A target is never
  treated as aged by default.
- Known limits (accepted with D9):
  - A patient attacker who already controls a Live wallet (which needs an unchallenged fabricated DKG result,
    a separate issue outside this record) can wait out the creation period. The gate buys watchers one
    creation period to flag a rogue wallet.
  - Liveness cost: when no aged Live wallet has capacity, re-anchors wait until one ages, and the retiring
    wallet's anchors stay put in the meantime. Log it and add a metric for "re-anchor waiting on aged target"
    so operators can see it.
- Leads in this package: G1-L21 (a target's own pending re-anchor can self-block, untraced: monitor), G1-L24
  (cap-race burns operator gas per target, bounded by candidate count: dedupe request retries per window if
  adopted), G1-L25 (send without a final on-chain re-read; same window as the Run-level race: re-read the
  action state before broadcast if adopted), G1-L27 (the below-dust check runs only on the zero-reservation
  path; mirror it in the re-anchor assembler as hardening, G1-F5 territory).
- Regression tests:
  - Follower: new `TestReservationReanchorAction_RejectsYoungTarget` in `pkg/tbtc`. The case table covers a
    young target (rejected, no signing), an aged target (signs), a target exactly at the boundary within the
    2-block tolerance (signs), and a head-read or wallet-read failure (rejected).
  - Leader: new `TestReservationReanchorTask_SkipsYoungResumeTarget` (young target: no
    `ProposeReservationReanchor` call; aged target: resume proceeds) and `TestReanchorTargetSearch_SkipsYoungCandidate`
    (young candidate excluded from `pick`, aged candidate picked).
  - Fail closed: `TestReanchorCustodyGate_ChainHeadReadFailureFailsClosed` (head read error: both leader paths
    skip and retry on the next pass).
- Acceptance check: a follower never co-signs a re-anchor to a target younger than the on-chain creation
  period, even when the leader proposes one directly; an honest leader never proposes such a target; a read
  failure means no signature; the stall case is visible in logs and metrics.

### P4. RPC deadlines in the reservation execute paths (Medium; G1-F11)

- ID: G1-F11 (Medium, Plausible).
- Location: `pkg/tbtc/reservation.go`, `reservationAnchorAction.execute` (:497-531: `GetTransaction` :500,
  `GetDepositRequest` :505, `FindDepositRevealedEventByRevealTime` :528, `ValidateReservationAnchorProposal`
  :567) and `reservationReanchorAction.execute` (:692-717: `GetReservation` :695, `GetReservationAction`
  :700, `ValidateReservationReanchorProposal` :711). The per-wallet dispatcher lock (`wd.actions[key]`) is
  held from map store until the defer removal after the goroutine returns.
- Change: wrap the on-chain validator calls and the Bitcoin RPC reads in a bounded `context.WithTimeout`
  (30 s, consistent with the `HeaderByNumber` current-block pattern already used in the codebase); on
  timeout, release the dispatcher lock and return an error.
- Regression test: new `TestReservationExecute_RPCTimeoutReleasesDispatcher` in `pkg/tbtc`: a fake chain that
  hangs on the validator view and `GetTransaction`; `execute` returns within ~30 s and the dispatcher accepts
  the next action for the same wallet.
- Acceptance check: a hung Ethereum RPC endpoint no longer freezes every signing action for a wallet
  indefinitely; the operator sees a per-action timeout error and the wallet keeps working.

### P5. Pprof gating on the metrics port (Medium; G5-F1, G5-F2)

- IDs: G5-F1 (Medium, Confirmed), G5-F2 (Low, Confirmed).
- Location: `pkg/clientinfo/clientinfo.go` (named `net/http/pprof` import at :6, `Initialize` gate at :49-58,
  `registerPprofHandlers` at :69-75), keep-common v1.7.1-tlabs.0 `EnableServer`, and `docs/profiling.md`
  (:14-19 false claim, :32-43 documented flag).
- Change: delete the named `net/http/pprof` import entirely. Its `init()` registers `/debug/pprof/*` on
  `http.DefaultServeMux` unconditionally, so with `EnablePprof=false` the endpoints are live on the
  all-interfaces metrics port, and with `EnablePprof=true` the re-registration panics at startup (G5-F2).
  Mount the pprof handlers on a private `http.ServeMux` passed to `http.Server.Handler`, only when
  `EnablePprof` is true. Until keep-common accepts a custom handler, the minimal in-repo mitigation is to
  delete the import and the `registerPprofHandlers` calls and accept that pprof is unavailable. Correct
  `docs/profiling.md` either way.
- Regression test: invert `TestPprofSecondCheck` (evidence/g5, 12-endpoint capture under
  `evidence/g5/pprof-dump/`) to assert all 12 `/debug/pprof/*` paths return 404 with `EnablePprof=false` and
  the secret-bytes scan stays clean. Add `TestPprofEnabled_InitializesWithoutPanic`:
  `Initialize(Config{EnablePprof: true})` returns without panicking (converts the G5-F2 `go run` panic repro
  into a permanent test).
- Acceptance check: flag off, no `/debug/pprof/*` endpoint is served on the metrics port; flag on, the node
  starts and pprof is reachable on the intended mux.

### P6. Ephemeral key shape validation (Medium; G4-F8, G4-F9)

- IDs: G4-F8 (Medium, Confirmed), G4-F9 (Medium, Confirmed).
- Location: `pkg/tecdsa/dkg/marshaling.go` `unmarshalPublicKeyMap` (validates member-index keys only, accepts
  arbitrary-length `[]byte` values) and the skip-branch in `pkg/tecdsa/dkg/protocol.go` :84-104; identical
  twins in `pkg/tecdsa/signing/marshaling.go` and `pkg/tecdsa/signing/protocol.go` :84-104.
- Change: validate each ephemeral key entry's 33-byte compressed-point shape at unmarshal time, failing the
  whole message on any bad entry. That restores the pre-PR uniform-drop behavior: every receiver drops the
  message, so one corrupt targeted entry can no longer split the group's active view.
- Regression test: convert `TestSecondCheck_F8` (second check against the real `unmarshalPublicKeyMap`; it
  showed `map[uint32][]byte{2:{0x02},3:{0x02}}` was accepted) into a permanent `pkg/tecdsa/dkg` test
  asserting rejection, and `TestSecondCheck_F9` into the matching `pkg/tecdsa/signing` test.
- Acceptance check: a corrupt targeted ephemeral key entry fails wire-layer unmarshal for every member; the
  DKG and signing groups keep a uniform active view.

### P7. Eviction and unbounded growth in the proof loops (Medium; G2-F1, G2-F16; lead G2-L10)

- IDs: G2-F1 (Medium, Confirmed), G2-F16 (Low, Plausible).
- Location: `pkg/maintainer/spv/reservation_proof_loop.go`, `settleableReanchorReservation` (:680-719, the
  deliberate no-age-eviction comment at :678), `pendingAcceptanceEvents` retention (:419-477, the TimedOut
  eviction at :445-448 leaves Pending unevicted at :471-475), the backoff cache-refresh gate (:760-766,
  :938-944); plus `reservation_action_timeout_watch.go` :376-379 (stuck pending action entries).
- Change:
  - G2-F1: bound re-anchor retention. Evict a TimedOut re-anchor generation after a configurable window
    (multiple of the request timeout; the Bridge settles re-anchors unbounded, so other operators and later
    paths still cover it), or cap tracked generations per reservation to the newest N plus pending.
  - G2-F16: evict a Pending acceptance generation after a configurable age, or gate tracking on the
    action-timeout watcher's health.
  - G2-L10: evict stuck pending action entries after a bounded time.
- Regression test: invert the existing `TestProveReservationReanchorActions_TimedOutSourceAnchor` (subtest
  `unchanged_source_anchor_is_kept_regardless_of_age`, which pins indefinite retention) and
  `TestProveReservationReanchorActions_TimedOutUnbounded` to assert bounded eviction; invert
  `TestG2F16_StuckPendingAcceptanceGenerationTracksForever` (evidence/g2, currently proves one read per pass
  for the process life) to assert age-bounded eviction; update
  `TestProveReservationReanchorActions_TimedOutBackoff` for the cache refresh.
- Acceptance check: a permissionless requester cycling request-then-timeout against a target wallet no longer
  grows per-pass RPC volume without bound; the event maps stay bounded.

### P8. Chain time vs wall clock (Low; G1-S1, G1-S2; leads G1-L28, G2-L7, G2-L11, G2-L13)

- IDs: G1-S1 (Low, Plausible), G1-S2 (Low, Plausible). This is the Go-side half of x-ray invariant I-11 (the
  only Enforced=No invariant), which the scan scoped to the G2 watcher files.
- Locations: `pkg/tbtcpg/reservation_reanchor.go` :288 and :344 (`uint64(time.Now().Unix())` vs
  `action.TimeoutAt` and `reservation.ReanchorCooldownUntil`); `pkg/tbtcpg/reservation_acceptance.go` :433
  (`now := time.Now()`), :453 and :529 (`now.Unix()` vs `requestEvent.TimeoutAt` / `action.TimeoutAt`); G2
  leads: `reservation_proof_loop.go` :445 (G2-L7), `reservation_action_timeout_watch.go` :426/:596 (G2-L11),
  `reservation_stale_deposit_watch.go` :262 (G2-L13).
- Change: read the latest on-chain block timestamp (bounded 30 s `HeaderByNumber` read, a pattern the
  codebase already uses) and compare `timeoutAt`, `reanchorCooldownUntil`, and the stale-deposit refund
  deadline against it instead of the host wall clock. A fast host clock currently makes the node skip a
  generation the on-chain validator would still accept; a slow host clock makes it propose one the validator
  rejects (guaranteed revert plus operator gas).
- Regression test: update `TestProveReservationAcceptanceActions_TimedOutLateWindow` (currently proves the
  eviction fires from a local `nowFn` with zero chain-timestamp reads in the path) to a chain-timestamp gate;
  update `TestL28_ReanchorCooldownGate_HasNoSafetyMargin` (evidence/g1, pins the exact gate expression at
  `reservation_reanchor.go:344`) to the chain-timestamp form; add a skewed-clock test asserting no
  guaranteed-revert proposals.
- Acceptance check: all four gate sites read the chain timestamp; host clock skew no longer causes skipped
  generations or guaranteed reverts.

### P9. Scan cost in the SPV maintainers (Low; G2-F3, G2-F4, G2-F5, G2-F10, G2-F11)

- IDs: G2-F3, G2-F4, G2-F5 (Low, Plausible); G2-F10, G2-F11 (Low, Confirmed).
- Locations: `reservation_stale_deposit_watch.go` `pollTick` (:567-594, per-tick read after each
  past-deadline record); `reservation_wiring.go` `resolveWalletPublicKeyHash` (:759-767,
  `WalletRegisteredEventFilter` with no `StartBlock`, i.e. genesis scan); `reservation_proof_loop.go`
  `walletTransactionsForProof` (:172-174, newest-`config.TransactionLimit` = 20 window);
  `pkg/chain/ethereum/tbtc.go` `WalletReservations` (:1326-1329) and `WalletTerminationCause` (:1077-1080),
  both falling back to `startBlock = 0` on a transient `earliestWalletRegistrationBlock` (:1383) error.
- Change:
  - G2-F3: cap per-tick reads with a bounded budget and round-robin across tracked deposits; a notified
    past-deadline record is not re-read every tick until its state changes.
  - G2-F4: bound the registration filter's `StartBlock` with `earliestWalletRegistrationBlock` instead of
    genesis.
  - G2-F5: for TimedOut re-anchor generations, fetch the expected outpoint directly instead of the newest-20
    transaction window.
  - G2-F10/F11: on a transient `earliestWalletRegistrationBlock` error, return the error and defer to the next
    pass; never degrade to the chunked block-zero scan (10,000-block `eth_getLogs` chunks most providers
    reject).
- Regression test: invert `TestG2F3_StaleDepositPollTickReadsScaleWithTrackedCount` (reads now capped per
  tick), `TestG2F4_CloseHandlerResolutionFailureDropsTheCheck` subtest 1 (filter `StartBlock != 0`), and
  `TestG2F5_TransactionLimitTruncatesOldReanchorProof` (a TimedOut generation beyond the newest-20 window still
  gets its proof by outpoint fetch). New `TestWalletReservations_TransientRegistrationErrorFailsPass` for
  G2-F10/G2-F11 (error propagates, `startBlock = 0` never happens).
- Acceptance check: per-tick RPC reads are bounded; no genesis-wide `eth_getLogs` on a transient error; a late
  re-anchor proof survives more than 20 subsequent wallet transactions.

### P10. Error isolation in the reservation watchers (Low; G2-F8, G2-F9, G2-F15, G2-F17, G2-F22; leads G2-L2, G2-L5, G2-L15)

- IDs: G2-F8 (Low, Plausible), G2-F9, G2-F15, G2-F17, G2-F22 (Low, Confirmed).
- Locations: `reservation_proof_loop.go` `proveReservationTransaction` (:1121-1131, first `getProofInfo` error
  returns `reservationProofNoMatch` for the whole candidate list); `pkg/chain/ethereum/tbtc.go`
  `resolveCustodiedReservationKeys` (:1546-1554, first `GetReservation` error aborts the wallet-wide lookup);
  `reservation_proof_loop.go` :563-565 and :1000-1002 (a failed submit still claims the reservation key);
  `reservation_stranding_watch.go` `checkReservationStrandingForWallet` (:149-167, silent `continue` after
  retries); leads: `reservation_stale_deposit_watch.go` :383-396 (G2-L2), `reservation_proof_loop.go`
  :264-276 (G2-L5), `reservation_wiring.go` :740-750 (G2-L15).
- Change:
  - G2-F8: `continue` past a per-candidate `getProofInfo` error so later confirmed RBF replacements are still
    examined.
  - G2-F9: log and `continue` past a per-key read error in `resolveCustodiedReservationKeys` instead of
    `return`.
  - G2-F15/F17: mark `submittedReservations[reservationKey]` only when `err == nil`.
  - G2-F22: requeue the wallet for a later stranding pass on a failed read instead of silently continuing.
  - G2-L2: gate stale-deposit tracking on the action-timeout watcher's health and add a self-recovery path for
    a dead watcher.
  - G2-L5: let transient chunk-fetch errors take the idle backoff (with progress preserved) instead of
    `RestartBackoffTime`.
  - G2-L15: retry or requeue the failed stranding check from the close handler instead of dropping it.
- Regression test: invert `TestG2F8_EarlierCandidateConfirmationsErrorHidesLaterReplacement` (the confirmed
  replacement is examined and submitted), `TestG2F9_ResolveCustodiedReservationKeysAbortsOnFirstLookupError`
  (evidence/g2 `tbtc_f9_evidence_test.go.txt`; queried keys become `[1, 3]`),
  `TestG2F15_FailedSubmissionBlocksSiblingGeneration` and
  `TestG2F17_FailedReanchorSubmissionBlocksSiblingGeneration` (both generations are attempted in one pass), and
  `TestG2F22_SingleReservationReadFailureDropsOnlyThatKey` (the failing key's wallet is requeued). Update
  `TestProveReservationAcceptanceActions_ChunkedScan` (transient chunk error takes the idle backoff) and
  `TestG2F4_CloseHandlerResolutionFailureDropsTheCheck` subtest 2 (the failed check is retried or requeued).
- Acceptance check: one failed read or one failed submit no longer starves sibling generations, sibling keys,
  or the rest of a wallet's reservations; a transient chunk error does not stall the proof loop for 30
  minutes.

### P11. Reservation flag wiring (Low; G4-F6, G2-F24, G2-F25)

- IDs: G4-F6 (Low, Confirmed), G2-F24 (Low, Plausible), G2-F25 (Low, Confirmed).
- Locations: `cmd/start.go` :236-243 (the paired-flag branch: `Tbtc.ReservationsEnabled` on plus
  `Maintainer.Spv.ReservationProofsEnabled` off only logs a warning; the start-process config has no
  `[maintainer.spv]` section in a split deployment, so a hard error would reject valid splits);
  `reservation_wiring.go` :324-327 and :692-693 (startup scan and retry pass forward only Closed/Terminated
  wallets); `reservation_wiring.go` :599-601 (`reservationSelfCheckMisconfigured` requires
  `registration.count == 0` in addition to the two reservation scans being zero, which never happens on an
  active network).
- Change:
  - G4-F6: keep startup non-fatal; escalate on the client side. The action-timeout watcher alerts (metric plus
    operator log, not just the on-chain notice) when a reservation action has been pending without a
    corresponding SPV proof submission for N blocks, and ship a deploy-check that compares the two flags across
    the start and maintainer config files.
  - G2-F24: include dissolution-eligible Closing wallets (`block.timestamp >= dissolutionEligibleAt`,
    `bridge/Reservation.sol:1106-1113`) in the set forwarded to the stranding check, in both the startup scan
    and the retry pass.
  - G2-F25: drop the `registration.count == 0` term so the self-check keys only on the reservation-specific
    scans.
- Regression test: convert `TestF6_WireReservationWatchers_PairedFlagOff_IsNonFatal` (evidence/g4, currently
  proves non-fatal wiring plus the quiet-network shape that masks the misconfiguration) to assert the
  escalation signal: paired flag off plus a pending action with no proof submission for N blocks fires the
  alert, and the deploy-check reports the cross-config mismatch. Invert
  `TestG2F24_StartupStrandingScanNeverChecksClosingWallet` (a Closing-eligible wallet is now notified) and
  `TestG2F25_SelfCheckKeysOnRegistrationCount` (a non-zero registration count no longer masks the signal).
- Acceptance check: a mismatched paired flag produces an operator-visible alert on a quiet network, not just a
  log line; a dissolution-eligible Closing wallet gets its stranding check; the self-check fires on an active
  network.

### P12. Periphery fixes (Low; G5-S2, G5-S3, G5-S6)

- IDs: G5-S2 (Low, Confirmed), G5-S3 (Low, Confirmed), G5-S6 (Low, Plausible).
- Locations: `pkg/net/local/broadcast_channel_manager.go` `getBroadcastChannel` (:45-46, per-creation
  retransmission ticker on a Background context), `broadcastMessage` (:66-73, name-keyed lookup),
  `ReleaseBroadcastChannel` (:84-93, cancel is not a barrier); `pkg/chain/ethereum/ethereum.go`
  `gasEstimateWithMargin` (:537-540, `uint64(float64(gasEstimate) * 1.2)` truncates the margin to 0 percent for
  estimates of 4 units or less; used at `pkg/chain/ethereum/bitcoin_difficulty.go:298`);
  `pkg/beacon/dkg/marshaling.go` `ThresholdSigner.Unmarshal` (:52, :65, the `GroupPrivateKeyShare` decimal
  string has no curve-order bound before `CalculateSignatureShare`).
- Change:
  - G5-S2: make release a barrier. Give the channel list a generation counter or released flag; a ticker in
    flight captures its session's list at fire time and checks the flag, so an old session's message (old
    sender transport ID, old sequence number) is never delivered into the next session's channels.
  - G5-S3: compute the margin in integer math with ceiling (for example
    `gasEstimate + (gasEstimate*20 + 99) / 100`) so the 20 percent margin is exact even at small gas values.
  - G5-S6: reject a deserialized `GroupPrivateKeyShare` that is zero or at or above the curve order, before
    any signature share use.
- Regression test: new `TestLocalBroadcastRelease_BarriersInFlightRetransmit` (an in-flight tick during release
  never reaches the next session's channels), `TestGasEstimateWithMargin` (g = 1..4 now carries the full 20
  percent margin; g = 54000 stays 64800), and `TestThresholdSigner_Unmarshal_OutOfRangeShare` (shares at or
  above the curve order are rejected at unmarshal).
- Acceptance check: no cross-session retransmission after release; the gas margin is exact at small estimates;
  out-of-subgroup key shares never reach `CalculateSignatureShare`.

## Item table

All 36 PR-introduced Confirmed/Plausible findings, each exactly once.

| ID | Sev | Verdict | Package | Location | Fix (one line) | Regression test |
|---|---|---|---|---|---|---|
| G2-F2 | High | Confirmed | P1 | reservation_action_timeout_watch.go:575-579 (checkReservationActionTimeout zero-wallet skip) | Remove the zero-wallet early return; the acceptance notice needs only the reservation key; keep the hash only for the reanchor re-queue | Convert TestG2F2_ZeroWalletPendingAcceptance_SkipsAcceptanceTimeoutNotice (evidence/g2) to expect 1 notice |
| G2-F18 | Low | Plausible | P1 | reservation_action_timeout_watch.go:680-682 (reanchor stranding re-queue) | Enqueue strandingRecheckWallets regardless of the notice outcome | Invert TestG2F18_FailedReanchorTimeoutNoticeSkipsStrandingRequeue |
| G2-F19 | Low | Plausible | P1 | reservation_action_timeout_watch.go:746-748 (drainStrandingRechecks non-terminal skip) | Requeue the wallet on the Closing/still-pending branch instead of dropping it | Invert TestG2F19_DrainDropsClosingWalletAndCleanPendingSkip |
| G1-F7 | Medium | Confirmed | P2 | pkg/tbtc/reservation.go:579-592 (reservationAnchorAction.execute fee check) | Hard reject when fee < ProtocolMinWalletTxSatPerVByteFee * (vsize - 1); warning stays at the bare floor (MinWalletTxSatPerVByteFee * vsize) | New per-path protocol-min tests (fee = ProtocolMin*(vsize-1)-1 rejected; fee = ProtocolMin*vsize signs) plus the leader-vs-follower cross-check table test |
| G1-F8 | Medium | Confirmed | P2 | pkg/tbtc/reservation.go:719-732 (reservationReanchorAction.execute fee check) | Same protocol-minimum reject and bare-floor warning on the re-anchor fee | Same per-path and cross-check tests, re-anchor instance |
| G1-F6 | Medium | Confirmed | P2 | pkg/tbtc/deposit_sweep.go:258-265 (depositSweepAction.execute warn-only sweep fee floor, checkSweepFeeFloor) | Same protocol-minimum reject and bare-floor warning on the sweep fee | Same per-path and cross-check tests, sweep instance |
| G4-F10 | Low | Confirmed | P2 | pkg/tbtc/moving_funds.go:380-389 (ValidateMovingFundsProposal warn-only fee floor) | Same protocol-minimum reject and bare-floor warning on the moving-funds fee | Same per-path and cross-check tests, moving-funds instance |
| G4-F11 | Low | Confirmed | P2 | pkg/tbtc/redemption.go:401-413 (ValidateRedemptionProposal warn-only fee floor) | Same protocol-minimum reject and bare-floor warning on the redemption fee | Same per-path and cross-check tests, redemption instance |
| G1-S3 | Low | Confirmed | P2 | pkg/tbtc/proposal_fee_check.go (warnIfProposedWalletTxFeeBelowBufferedFloor compares against the buffered floor, which honest low-fee leaders land under) | Move the warning to the bare floor (MinWalletTxSatPerVByteFee * vsize); the reject is the protocol minimum | Convert the throwaway evidence/g1/fee_floor_leader_vs_follower_test.go table into a cross-check test asserting no honest leader output is rejected |
| G1-F2 | Medium | Plausible | P3 | pkg/tbtc/reservation.go:692-777 (follower reservationReanchorAction.execute, no target check) and pkg/tbtcpg/reservation_reanchor.go:308 (resume) | Follower rejects a target where CreatedAt + walletCreationPeriod > chain head time + 2 blocks; leader resume skips un-aged targets (strict); fail closed | New TestReservationReanchorAction_RejectsYoungTarget (follower) and TestReservationReanchorTask_SkipsYoungResumeTarget (leader) |
| G1-F12 | Medium | Plausible | P3 | pkg/tbtcpg/reservation_reanchor.go:950-1002 (reanchorTargetSearch.pick) | Exclude pick candidates younger than the on-chain creation period (strict, chain head time); the follower check from G1-F2 backs it; fail closed on a read error | New TestReanchorTargetSearch_SkipsYoungCandidate plus the fail-closed test |
| G1-F11 | Medium | Plausible | P4 | pkg/tbtc/reservation.go:497-531, :692-717 (execute-path RPC calls under the dispatcher lock) | Wrap validator view calls and Bitcoin RPC reads in a bounded 30 s context; release the dispatcher lock on timeout | New TestReservationExecute_RPCTimeoutReleasesDispatcher |
| G5-F1 | Medium | Confirmed | P5 | pkg/clientinfo/clientinfo.go:6,49-58 (named pprof import plus inert flag gate) | Delete the net/http/pprof import; mount pprof on a private mux only when EnablePprof is true; fix docs/profiling.md | Invert TestPprofSecondCheck (evidence/g5) to expect 404 on all 12 endpoints |
| G5-F2 | Low | Confirmed | P5 | pkg/clientinfo/clientinfo.go:69-75 (registerPprofHandlers double registration panics) | Same import removal stops the startup panic; flag on now works | New TestPprofEnabled_InitializesWithoutPanic |
| G4-F8 | Medium | Confirmed | P6 | pkg/tecdsa/dkg/marshaling.go (unmarshalPublicKeyMap key-only validation) plus dkg/protocol.go:84-104 skip-branch | Validate each ephemeral key entry as a 33-byte compressed point at unmarshal; fail the whole message | Convert TestSecondCheck_F8 to a permanent pkg/tecdsa/dkg test |
| G4-F9 | Medium | Confirmed | P6 | pkg/tecdsa/signing/marshaling.go plus signing/protocol.go:84-104 (byte-identical twin) | Same unmarshal-time shape validation on the signing side | Convert TestSecondCheck_F9 to a permanent pkg/tecdsa/signing test |
| G2-F1 | Medium | Confirmed | P7 | reservation_proof_loop.go:680-719 (settleableReanchorReservation) | Evict abandoned TimedOut re-anchor generations after a configurable window, or cap generations per reservation | Invert TestProveReservationReanchorActions_TimedOutSourceAnchor and ..._TimedOutUnbounded |
| G2-F16 | Low | Plausible | P7 | reservation_proof_loop.go:421-477 (Pending acceptance retention) | Evict a stuck Pending acceptance generation after a configurable age, or gate on watcher health | Invert TestG2F16_StuckPendingAcceptanceGenerationTracksForever (evidence/g2) |
| G1-S1 | Low | Plausible | P8 | pkg/tbtcpg/reservation_reanchor.go:288,344 (wall clock vs TimeoutAt and ReanchorCooldownUntil) | Compare against the latest on-chain block timestamp instead of time.Now() | Update TestL28_ReanchorCooldownGate_HasNoSafetyMargin (evidence/g1) plus new skewed-clock test |
| G1-S2 | Low | Plausible | P8 | pkg/tbtcpg/reservation_acceptance.go:433,453,529 (now := time.Now() vs TimeoutAt) | Same chain-timestamp gate in the acceptance candidate finder | New chain-timestamp gate test for findReservationAcceptanceCandidates |
| G2-F3 | Low | Plausible | P9 | reservation_stale_deposit_watch.go:567-594 (pollTick per-tick reads) | Cap per-tick reads; round-robin budget across tracked deposits; no re-read of a notified record until state changes | Invert TestG2F3_StaleDepositPollTickReadsScaleWithTrackedCount |
| G2-F4 | Low | Plausible | P9 | reservation_wiring.go:759-767 (resolveWalletPublicKeyHash genesis-scan filter) | Bound the filter's StartBlock with earliestWalletRegistrationBlock | Invert TestG2F4_CloseHandlerResolutionFailureDropsTheCheck subtest 1 |
| G2-F5 | Low | Plausible | P9 | reservation_proof_loop.go:172-174 (walletTransactionsForProof newest-20 window) | For TimedOut re-anchor generations, fetch the expected outpoint directly instead of the newest-N window | Invert TestG2F5_TransactionLimitTruncatesOldReanchorProof |
| G2-F10 | Low | Confirmed | P9 | pkg/chain/ethereum/tbtc.go:1326-1329 (WalletReservations startBlock = 0 fallback) | On a transient registration-lookup error, fail the pass and defer to the next one; never scan from block zero | New TestWalletReservations_TransientRegistrationErrorFailsPass |
| G2-F11 | Low | Confirmed | P9 | pkg/chain/ethereum/tbtc.go:1077-1080 (WalletTerminationCause startBlock = 0 fallback) | Same: propagate the transient error instead of the genesis-wide chunked scan | New TestWalletTerminationCause_TransientRegistrationErrorFailsPass |
| G2-F8 | Low | Plausible | P10 | reservation_proof_loop.go:1121-1131 (proveReservationTransaction first-error NoMatch) | continue past a per-candidate getProofInfo error; later confirmed replacements are still examined | Invert TestG2F8_EarlierCandidateConfirmationsErrorHidesLaterReplacement |
| G2-F9 | Low | Confirmed | P10 | pkg/chain/ethereum/tbtc.go:1546-1554 (resolveCustodiedReservationKeys first-error return) | Log and continue past a per-key read error so one bad key does not starve the wallet's other reservations | Invert TestG2F9_ResolveCustodiedReservationKeysAbortsOnFirstLookupError (evidence/g2 tbtc_f9_evidence_test.go.txt) |
| G2-F15 | Low | Confirmed | P10 | reservation_proof_loop.go:563-565 (acceptance pass claims the key on a failed submit) | Mark submittedReservations[key] only when err == nil | Invert TestG2F15_FailedSubmissionBlocksSiblingGeneration |
| G2-F17 | Low | Confirmed | P10 | reservation_proof_loop.go:1000-1002 (reanchor pass claims the key on a failed submit) | Mark submittedReservations[key] only when err == nil | Invert TestG2F17_FailedReanchorSubmissionBlocksSiblingGeneration |
| G2-F22 | Low | Confirmed | P10 | reservation_stranding_watch.go:149-167 (checkReservationStrandingForWallet silent continue) | Requeue the wallet for a later stranding pass on a failed read instead of silently dropping that key | Invert TestG2F22_SingleReservationReadFailureDropsOnlyThatKey |
| G4-F6 | Low | Confirmed | P11 | cmd/start.go:236-243 (paired-flag warn-only branch) | Keep non-fatal; add the client-side alert for a pending action with no SPV proof for N blocks plus a cross-config deploy-check | Convert TestF6_WireReservationWatchers_PairedFlagOff_IsNonFatal (evidence/g4) to assert the escalation signal |
| G2-F24 | Low | Plausible | P11 | reservation_wiring.go:324-327,692-693 (forwarding only Closed/Terminated) | Include dissolution-eligible Closing wallets in the forwarded set | Invert TestG2F24_StartupStrandingScanNeverChecksClosingWallet |
| G2-F25 | Low | Confirmed | P11 | reservation_wiring.go:599-601 (reservationSelfCheckMisconfigured registration.count == 0 term) | Drop the registration-count term; key the self-check on the reservation-specific scans only | Invert TestG2F25_SelfCheckKeysOnRegistrationCount |
| G5-S2 | Low | Confirmed | P12 | pkg/net/local/broadcast_channel_manager.go:45-46,66-73,84-93 (release is not a barrier) | Generation counter or released flag per channel list; in-flight ticks of a released session never reach the next session's channels | New TestLocalBroadcastRelease_BarriersInFlightRetransmit |
| G5-S3 | Low | Confirmed | P12 | pkg/chain/ethereum/ethereum.go:537-540 (gasEstimateWithMargin float64 truncation) | Integer ceiling margin instead of uint64(float64(g) * 1.2) | New TestGasEstimateWithMargin (g = 1..4 carries the full 20 percent) |
| G5-S6 | Low | Plausible | P12 | pkg/beacon/dkg/marshaling.go:52,65 (ThresholdSigner.Unmarshal share has no curve-order bound) | Reject a deserialized GroupPrivateKeyShare that is zero or at or above the curve order | New TestThresholdSigner_Unmarshal_OutOfRangeShare |
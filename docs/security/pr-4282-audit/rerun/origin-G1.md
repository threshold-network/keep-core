# G1 origin reclassification (PR #4282 re-audit, merged fix head)

Ranges: base `a7ac8989`, epic/PR `49c4df13d` (diff `a7ac8989..49c4df13d`), PR fix `0b0c89cb1` (diff
`49c4df13d..` the audit-fix line), dev `d72bca6fc`. Hashes are 8-hex function-body hashes at head;
`git show <sha>:<file>` confirms the cited line. The scan labelled every item in this batch "Dev";
reclassification below overrides that label where the evidence differs. Items whose final origin is
pre-existing are not part of this record and are omitted.

## Items

| ID | scan origin | final origin | evidence (function + hash line or commit) | final verdict | one-line reason |
|---|---|---|---|---|---|
| G1-R11 | Dev | PR fix | `pkg/tbtc/reservation.go` `rpcCallWithTimeout`: file ABSENT at base; `git log -S 'rpcCallWithTimeout'` = a83ba04ef (in the audit-fix range only, not in base/epic/dev); the parking goroutine `go func() { val, err := call(); resultCh <- result{val, err} }()` + `select { ... case <-time.After(reservationRPCRequestTimeout): ... }` at `pkg/tbtc/reservation.go:36-60` | Confirmed | the P4 bounded-RPC helper: on timeout the spawned goroutine stays parked writing to the size-1 channel until the Electrum retry budget ends - unbounded goroutine leak per timed-out read, memory-bounded, liveness-only. Fixed by the global 64-slot bound in `106322562` |
| G1-R18 | Dev | PR | `pkg/tbtc/reservation.go` ReanchorTxFee truncation: `rra.proposal.ReanchorTxFee.Int64()` at `pkg/tbtc/reservation.go:972` feeds `AssembleReservationReanchorTransaction`'s `fee int64` param; `git log -S 'ReanchorTxFee.Int64()'` = 6f20acae8 (in epic range `a7ac8989..49c4df13d`, NOT in base/fixes/dev); the `reservation.go` file itself is ABSENT at base | held (Low) | the re-anchor follower truncates an unmarshaled `*big.Int` fee to `int64` before the assembler's `fee <= 0` / `uint64(fee) > TxMaxFee` guards; since the PR-added reservation unmarshal now caps fee bytes at 8, `Int64()` can no longer wrap for on-network proposals - residual only reachable via a programmatically-set out-of-range `ReanchorTxFee` |
| G1-R19 | Dev | PR fix | leader-side `pkg/tbtcpg/reservation_reanchor.go` `IsReanchorTargetAged(..., 0)` at `:368` (resume) and `:1101` (pick) = 5a7b125fd (audit-fix range only); follower-side `pkg/tbtc/reservation.go:905/914` `IsReanchorTargetAged(..., ReanchorTargetAgeTolerance)` = a83ba04ef (same range); the `ReanchorTargetAgeTolerance` const + `IsReanchorTargetAged` helper in `pkg/tbtc/reservation_target_age.go` (file ABSENT at base and epic) introduced d59e38fd4 | held (Low); change reverted | leader pick/resume age-gate uses tolerance `0` while the follower co-sign gate uses `ReanchorTargetAgeTolerance` (24 s), so a target sitting exactly at the cutoff can be refused by the leader yet co-signed by a follower one head read later. Decision D9 keeps it: a strict leader never proposes a target a lenient follower rejects. The worker change was reverted (`4a9ad6bfb`) |
| G1-R21 | Dev | PR | `pkg/tbtcpg/reservation_reanchor.go` `Run` head=c2d8a8dd base=da39a3ee(absent; file first added a195031bc "implement reservation re-anchor executor", in epic range, NOT in base); cap-revert/retry mechanics `isReservationCapRevertError` + `errReservationReanchorPostSubmitFailure` first appear 5019414f9 ("send at most one re-anchor request per Run pass", in epic, not in base/dev); run-path body at `pkg/tbtcpg/reservation_reanchor.go:460-525` | held (Low) | a target-cap revert after `ProposeReservationReanchor` paid gas reverts the request, and the `for { targets.exclude(target); continue }` loop re-issues a new request against the next candidate in the same pass - griefing gas spend bounded by the reservation timeout/candidate count, no fund movement; intended and tested |
| G1-R22 | Dev | PR | `pkg/tbtcpg/reservation_reanchor.go` `Run` floor-check `anchorValue <= params.ReservationTxMaxFee+params.ReservationMinAmount` at `:447` (uint64+uint64; both uint64 in `tbtc.ReservationParameters`, `pkg/tbtc/reservation.go:311-314`), introduced c49778607 ("skip re-anchor requests the cooldown or anchor floor would revert", in epic range, NOT in base/dev); `reservation_reanchor.go` absent at base | Confirmed, held (Low); change reverted | two uint64 values sum on the run path with no overflow guard; both are governance-set on-chain caps that will not approach uint64 max, so the wrap is practically unreachable |
| G1-R23 | Dev | PR | `pkg/tbtcpg/fee.go` estimateReservationFixedSizeTxFee head=94eb9f3a base=da39a3ee(absent; function first added 97054f3fb "additional reservation acceptance/reanchor fixes and coverage", in epic range, NOT in base/dev); the sibling `estimateCappedFee` NOW has `if totalFee < 0` at `pkg/tbtcpg/fee.go:245` (that guard was later added by 1b4db9206, dev-only), but `estimateReservationFixedSizeTxFee` at head `pkg/tbtcpg/fee.go:288` still only checks `uint64(totalFee) > txMaxFee` with no `totalFee < 0` | Confirmed, held (Low); change reverted | `estimateReservationFixedSizeTxFee` lacks the negative-estimate guard `estimateCappedFee` has; a negative estimate is only reachable through the estimator's own error path, which the floor check already rejects |
| G1-R28 | Dev | PR | `pkg/tbtc/coordination.go` ReservationsActivationBlock head=c6950742 base=da39a3ee(absent); `var reservationsActivationBlocks = map[ethereum.Network]uint64{}` (empty literal) at `pkg/tbtc/coordination.go:94` + the fall-through to `math.MaxUint64` at `:110-119`, first introduced 9cddee249 ("derive reservations activation block from ethereum network", in epic range, NOT in base/dev) | Confirmed (intentional design) | the activation map ships as an empty literal, so every public network (Mainnet, Sepolia, Unknown) resolves to `math.MaxUint64` - reservation actions are gated off on all real networks until an operator adds an explicit entry; the doc comment at `:84-89` says this is the intended staged rollout |
| G1-R29 | Dev | PR | `pkg/tbtc/deposit_reveal_lookup.go` FindDepositRevealedEventByRevealTime head=8743042b base=da39a3ee(absent; whole file first added f9cb95c55 "locate anchor deposit reveals from reveal time, not a 30-day window", in epic range, NOT in base/dev); `elapsed := now.Sub(revealedAt)` at `:113` where `now` is the caller-supplied host-clock `time.Now()` (reservation anchor execute call site `pkg/tbtc/reservation.go:629`) | held (Low); reveal-lookup part fixed in `d92646e8f` | the band-centering math (estimating the reveal block from `(now - revealedAt)/avgBlockTime`) is new PR code but mirrors the on-chain validator's own reveal-window semantics - the G1-S1 residual; a band miss only widens the scan, it cannot exclude a valid reveal |

## Counts by final origin (this batch)

- PR = 5 (R18, R21, R22, R23, R29)
- PR fix = 3 (R11, R19, R28 - R28 is PR code with an intentional-design verdict)
- Pre-existing = the rest of the batch; those items are not part of this record.

## PR / PR-fix items with final verdicts

- G1-R11 (PR fix): Confirmed - unbounded goroutine parked per timed-out `rpcCallWithTimeout` call (fix
  commit a83ba04ef); memory-bounded liveness leak. Fixed in `106322562`.
- G1-R18 (PR): held (Low) - `Int64()` truncation no longer wraps for network-unmarshaled fees after the PR's
  8-byte fee cap; reachable only via a programmatically-set out-of-range `ReanchorTxFee`.
- G1-R19 (PR fix): held (Low) - leader `tolerance=0` vs follower `ReanchorTargetAgeTolerance` (24 s)
  split-brain on a boundary-aged re-anchor target (5a7b125fd leader + a83ba04ef follower). Reverted by D9.
- G1-R21 (PR): held (Low) - cap-revert race re-spends a re-anchor request after gas spend (5019414f9),
  bounded griefing, no fund movement.
- G1-R22 (PR): Confirmed - unguarded uint64+uint64 floor check at `Run`:447 (c49778607); practically
  unreachable at governance caps.
- G1-R23 (PR): Confirmed - `estimateReservationFixedSizeTxFee` lacks the `totalFee < 0` guard
  `estimateCappedFee` has (function first added 97054f3fb in the PR range) - self-catching robustness
  asymmetry, held (Low).
- G1-R28 (PR): Confirmed - `reservationsActivationBlocks` ships empty (9cddee249), so every public network
  falls through to `math.MaxUint64`; intentional staged-rollout design, not a defect.
- G1-R29 (PR): held (Low) - reveal-lookup band centers on caller-supplied host-clock `now` (f9cb95c55); a
  miss only widens the scan, no false negative for a valid reveal. Centered on chain head time by
  `d92646e8f`.
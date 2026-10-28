# X-ray delta: Go pre-audit, first run vs merged head

Comparison of the Phase C re-run x-ray (head `9f99f364a`; report committed in this tree at `rerun/x-ray-go/`) with the first run (epic head `50dd97a4b` / code `49c4df13d`; the first-run x-ray report was an internal run record and is not part of this record). Plain English, hyphens, straight quotes.

## Verdict

- Old: EXPOSED. New: EXPOSED (unchanged tier).
- What changed underneath: the single unenforced invariant of the first run is fixed on every reservation path; two new unenforced wall-clock-vs-chain-time sites appear outside the re-pointed paths; Tests stays FRAGILE (fuzz coverage of the reservation unmarshalers still absent, 0 property-based harnesses); the pprof surface was removed and the follower fee floor became a hard reject. The verdict holds at EXPOSED because the tier is capped by Tests + the security-critical untested `ValidateReservationAnchorProposal` (`pkg/chain/ethereum/tbtc.go:477`, still a `TODO(test-coverage)` at the merged head), not by the wall-clock invariant alone.

## Invariant counts

| | First run (49c4df13d) | Merged head (9f99f364a) |
|---|---|---|
| Enforced Guards | 75 | 67 |
| Single-component | 17 | 19 |
| Cross-component | 8 | 8 |
| Economic | 4 | 4 |
| Enforced=No | 1 (I-11) | 2 (I-18, I-19) |

Guard count dropped because the re-run catalogs guards per current file and several first-run guard rows were absorbed by code reorganization (adapter split, proof-loop rewrites in `146e3206d`); the new target-age gate, protocol fee hard reject, abandon gate, and dissolution-eligible gate add rows (I-9, G-15, G-16, G-17, G-60, G-62, G-67).

## Old Enforced=No invariants - do they now hold?

The first run had exactly one: I-11, "all reservation watcher timeout and deadline checks compare operator host wall-clock time against contract timestamps without syncing against Ethereum block header timestamps" (old file: `reservation_action_timeout_watch.go:426,613`, `reservation_stale_deposit_watch.go:262`).

- I-11 now HOLDS for every reservation lifecycle path (verified on the merged head):
  - Timeout-notify gate: `reservation_action_timeout_watch.go:356` reads `headTime := reservationChainHeadTime(ratw.spvChain)`; line 461 skips the pass when the head is unreadable (`if headTime.IsZero() { continue }`); line 465 judges `headNow >= deadline` where `headNow := uint32(max(headTime.Unix(), 0))`. Host clock (`nowFn`, line 224/355) is used only for the 10-min renotify backoff (line 408) and the `notifiedAt` stamp (line 481).
  - Stale-deposit refund gate: `reservation_stale_deposit_watch.go:272-281` - zero-time skip (`return StaleDepositResolutionKeep, nil`) and `if headNow <= deadline` on chain head time.
  - Late-window eviction: `reservation_proof_loop.go:494-498` - TimedOut acceptance evicted only when the chain head passes `TimeoutAt + TermSeconds`; abandonment bound `reservationActionAbandoned` at lines 1166-1172 (chain head > `TimeoutAt + 4*(TimeoutAt-RequestedAt)`); re-anchor side at 768/939.
  - Dissolution-eligible stranding gate: `reservation_stranding_watch.go:177,256-258` - Closing-wallet form reads the chain head and defers until `headTime >= DissolutionEligibleAt`.
  - Generator side (acceptance/re-anchor): `reservation_acceptance.go:433` and `reservation_reanchor.go:227` read `GetLatestBlockTimestamp()` once per pass and the pass FAILS when the read fails; the 7200 s margin, cooldown, maturity, and target-age checks all use that head time.
  - Accessor: `pkg/chain/ethereum/tbtc_chain_time.go:15` (`TbtcChain.GetLatestBlockTimestamp`), 24 lines, new file added by the fix branch (`2a8c1085e`), wired into the spv.Chain interface (`pkg/maintainer/spv/chain.go:226`).
  - Fix commits: `ad0575e0e` (spv watchers/proof loop), `768707517` (tbtcpg generators), `146e3206d` (hardening + interface method).
- Residual host-clock uses by design: renotify backoff tokens (`reservation_action_timeout_watch.go:224`, `reservation_stale_deposit_watch.go:723`, `reservation_wiring.go:461`) - these are operator-internal retry pacing, not deadline-vs-contract-timestamp decisions, and the underflow guard (`now < item.notifiedAt` at line 408) makes clock adjustments fail safe.

## New Enforced=No invariants (merged head)

- I-18: moving-funds safety margin on host wall clock. `pkg/tbtc/moving_funds.go:477-481`: `safetyMarginExpiresAt := walletChainData.MovingFundsRequestedAt.Add(safetyMargin); if time.Now().Before(safetyMarginExpiresAt) { return fmt.Errorf("safety margin in force") }` - compares the node clock to an on-chain `MovingFundsRequestedAt` + 24 h/14 d margin. A fast host clock expires the margin early and lets a moving-funds transaction be proposed while last-minute deposits or other wallets' moved funds are still landing; `ValidateMovingFundsProposal` (on-chain, out of scope) is the backstop. This gate predates the first audit run (present at `49c4df13d`, lines 472-474) and was not among the deadlines the fix branch re-pointed.
- I-19: deposit-sweep generator maturity on host wall clock. `pkg/tbtcpg/deposit_sweep.go:223` (`timeNow := time.Now()`) and `:288` (`if !timeNow.After(matureAt)` where `matureAt := depositRequest.RevealedAt.Add(depositMinAge)`) - the sweep generator decides deposit minimum age on the node clock while the reservation generators in the same checklist window decide it on chain time (`768707517`). Asymmetric clock domains: a fast host clock proposes sweeps for deposits the on-chain `ValidateDepositSweepProposal` still considers too young (wasted round, then on-chain rejection), a slow clock misses mature deposits.

## Entry points / trust boundaries added or removed

Added by the fix branch (merged head only):
- `ethereum.TbtcChain.GetLatestBlockTimestamp()` - new permissionless read/view on the chain-time source feeding every reservation deadline gate (`tbtc_chain_time.go:15`); a lagging head only defers decisions, never advances them; zero time on error disables the decision for the pass.
- Follower re-anchor target-age gate - `pkg/tbtc/reservation.go:901-916` + `pkg/tbtc/reservation_target_age.go:20-28` (`IsReanchorTargetAged`, 24 s tolerance, chain head time): a new co-sign precondition on the re-anchor signing path.
- Protocol fee minimum hard reject - `pkg/tbtc/proposal_fee_check.go:183-204` (`rejectIfProposedWalletTxFeeBelowProtocolMin`), wired into all five wallet-tx follower paths (`reservation.go:713,950`, `deposit_sweep.go:705`, `moving_funds.go:377`, `redemption.go:399`): the follower fee check upgraded from warn-only to a two-layer policy (hard reject at 5 sat/vB protocol min + soft warn at the operator floor).
- Stranding dissolution-eligible gate - `reservation_stranding_watch.go:256-258` (Closing-wallet form waits for chain head >= `DissolutionEligibleAt` before notifying).
- Overdue-action alert - `reservation_action_timeout_watch.go:466,510-530` (Warn + `spv_reservation_action_overdue_total` metric on every overdue notify decision; `7944269c6`).
- Bounded out-of-band deploy check - `scripts/check-reservation-flags.sh` + `cmd/deploy_check_test.go` (`7c8269d6a`): cross-checks the two paired config files for the dual opt-in flags.
- Out-of-range unmarshal rejections - `pkg/tecdsa/dkg/marshaling.go`, `pkg/tecdsa/signing/marshaling.go` (`eb8c3d39f`: every ephemeral key map entry parsed as a 33-byte compressed secp256k1 point) and `pkg/beacon/dkg/marshaling.go:76` (`bc1a0aa10`: bn256 private key share must lie in `[1, order)`).

Removed by the fix branch:
- `clientinfo.registerPprofHandlers()` and the `EnablePprof` setting - pprof endpoints gone from the metrics server (`pkg/clientinfo/clientinfo.go`, `38f8673e4`; `docs/profiling.md` deleted). The old attack surface "exposed clientinfo port leaks /debug/pprof" no longer exists; a regression test pins that `/debug/pprof` is not served.

By dev (origin/dev tail of the merge, 195 commits): no entry point or trust boundary added or removed in the 73-file Go scope - the dev changes in scope are the ABI-regeneration commit (`2f100cf78`, generated bindings only) and shared-tooling churn; all behavioral deltas listed above are fix-epic commits. The on-chain contract side (out of scope for the Go x-ray) is byte-identical to `origin/dev` at the merged head.

## Paths

- New x-ray: committed in this tree at `rerun/x-ray-go/` (`x-ray.md`, `invariants.md`, `entry-points.md`).
- This delta: the text above.
- Old (first-run) x-ray: an internal run record, not part of this record.

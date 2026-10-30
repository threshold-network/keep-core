# 10 - Accepted risks register (M1 UTXO reservations)

Scope: M1 reservations only, both repos. Pinned commits: tbtc-v2 `reservations-upgrade` @ `9d2086965` (`tbtc-v2`), keep-core `reservations-epic` @ `f99ef96e3` (`keep-core`). The code is the source of truth. D31 (privileged hand-off plus acceptance reserve) is integrated on the reservations branches at tbtc-v2 `reservations-upgrade` @ `806b1106f` and keep-core `reservations-epic` @ `d40f6aadc`; those facts are cited with an explicit revision (`tbtc-v2@806b1106f:...`, `keep-core@d40f6aadc:...`), and an unqualified citation still means the original pin. Nothing is deployed on Sepolia or mainnet.

Reading rules. Each AR entry gives: a plain description, why it was accepted (the decision and its date where known), the bound on its impact, how it is monitored, and the revisit trigger (before mainnet or at M2). A decision recorded as "for testnet, fix before mainnet" is a pre-mainnet revisit, not an M2 one. "Governance" is the Bridge owner (Council Safe / Timelock mainnet, deployer Sepolia); "SPV maintainer" is the allow-listed caller of the two proof functions.

Decisions named here come from the audit records (`gap-inventory.md`, `testnet-readiness-plan.md`, `m1-integrator-due-diligence.md`, `testnet-drill-runbook.md`), which are published separately from this review set. Dates are those log entries.

---

## AR-01 D19: a third-party hold still uses the target's ordinary slot

- Description: at `9d2086965`, a permissionless third-party `requestReservationReanchor` reserves the target wallet's ordinary per-wallet count and amount capacity (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:878`), not just a hold count. The `maxThirdPartyReanchorHoldsPerWallet` cap limits how many third-party holds a target can have, but each hold also consumes a normal slot. So a griefer who fills a target's last free slot makes the honest re-anchor request to that target revert with "Wallet reservations cap exceeded" (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:887`); keep-core then tries the next Live wallet. At `806b1106f` this still holds for third-party requests: the target count and amount are added on every path (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:934-935`, `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:949-950`), and a third party into a full target reverts with "Wallet reservations cap exceeded" (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:931`). What changed is the privileged hand-off: a request from the reservation owner, an SPV maintainer, a proven source-wallet member or governance, off a source that is not Live, sets `skipTargetCaps` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`) and skips both target cap checks (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:928-933`, `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:942-948`). So a third-party hold no longer blocks that hand-off.
- Why accepted: decision #190 (human, 2026-10-08) kept the permissionless re-anchor rule and capped third-party holds per target instead of restricting callers; the remaining limit (a hold reusing the ordinary slot) was accepted for testnet only. Log: `gap-inventory.md` D19 (2026-10-09/10); `testnet-readiness-plan.md` T4.2 (2026-10-08, "remaining risk ... accepted for testnet (D19)").
- Bound on impact: the hold becomes timeout-eligible after the action duration, but a caller must submit cleanup. It can persist longer without that transaction. It moves no Bitcoin itself. At `9d2086965` prolonged loss of usable targets could block a retiring source. At `806b1106f` a privileged hand-off off a MovingFunds or Closing source still succeeds into the occupied target (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:553`), and the privileged member request also succeeds after the hold times out (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:565`). The slot loss still blocks third-party requests (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:466`) and a governance rotation off a Live source, which keeps both caps (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:485`). The hold sits on a different position's key; a pending action on the same key still blocks a new request ("Reservation is not active", `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:783`).
- Monitoring: `pendingThirdPartyReanchorHolds(wallet)` view (`tbtc-v2:solidity/contracts/bridge/IReservationBridge.sol:563`); the drill runbook records any "Wallet reservations cap exceeded" revert as R1 (`testnet-drill-runbook.md:74`).
- Revisit trigger: D31 selected privileged hand-off plus an acceptance reserve, and it is integrated at `806b1106f` (see [approved design deltas](13-approved-design-deltas.md)). The remaining limit is that a third-party hold still uses the target's ordinary slot for third-party and Live-source requests. Revisit before mainnet whether that limit stays acceptable once the Sepolia drill has run. Not deployed on Sepolia or mainnet; the mainnet fork run (a local mainnet-fork run (no pin)) exercised third-party limits and a privileged above-cap hand-off.
- Status: `holds` (the hold counter and its release are in code and unit-tested, `tbtc-v2:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:386` at `9d2086965`; at `806b1106f` the privileged hand-off past an occupied target is pinned by `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:553` and `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:565`).

## AR-02 No M1 exit for the owner

- Description: M1 has no owner-initiated redemption, dissolution or closing of an accepted position. Re-anchor preserves global occupancy. Stranding can release it; acceptance timeout releases a pending request, not an accepted position. D26 release applies only before any acceptance request. An M1 position that has not migrated can end only by stranding, and an M1 strand does not fold the anchor into the main UTXO, so a wallet closing after a strand abandons the coins.
- Why accepted: the 2026-08-21 decision chose variant B over A+ precisely to remove dissolution (which opened a slashing attack on honest operators); the price is no owner exit. The 2026-10-29 product decision confirms the rejection of Option B and records the two accepted consequences. Log: `m1-guide.md` section 6; `exit/README.md` (emergency exit rejected on cost, 2026-08-21).
- Bound on impact: an accepted position may persist beyond its term because expiry is not an exit transaction. Holders can trade or redeem fungible TBTC through ordinary pool paths, subject to that pool's availability. The reservation's Bitcoin remains under the wallet key, not under an owner refund key.
- Monitoring: none on-chain; it is a product-limitation fact, not an alarm.
- Revisit trigger: M2 (redemption, renewal, dissolution and the owner exit ship together).
- Status: `holds` (the absence is enforced by what is not in code: no redemption, renewal, or dissolution entry point; `ActionType` keeps `Redemption` and `Dissolution` for storage layout only, `tbtc-v2:solidity/contracts/bridge/Reservation.sol:110`).

## AR-03 The reservation vault is not upgradeable

- Description: the M1 vault is a plain `Ownable`, not a proxy, and cannot be upgraded or re-pointed while any reservation exists. `receiveBalanceIncrease`, `financeInKindFee`, `sweepFees`, and the fee setters are fixed at deploy. A code change to the vault needs a new vault plus a Bridge-level vault re-point that requires the system to be empty.
- Why accepted: the "Option B minimal vault" decision (2026-09-24) deliberately keeps the M1 vault small and non-upgradeable so M2 can ship a separate vault without carrying an upgrade surface. Log: `m1-guide.md` section 5 (vault: "Once any reservation exists, it cannot be replaced").
- Bound on impact: the vault has no in-place upgrade. Re-pointing requires zero reserved amount and zero pending deposits. Deactivation stops new reserved reveals but does not close accepted positions. An accepted position cannot be cleared by a re-anchor timeout (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1409-1414`).
- Monitoring: none specific; it is a lifecycle fact. `isVaultTrusted(vault)` and the vault's `inKindFeeDebtSat` are the observable state.
- Revisit trigger: M2 (the new vault ships there).
- Status: `holds` (the `Ownable` base and the empty-system re-point precondition are in code, `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:46`, `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1409`).

## AR-04 M2 migration ceremony

- Description: because the M1 vault cannot be upgraded, M2 requires a new vault plus a depositor opt-in migration, and a Bridge implementation change to the vault-binding rule. M1-era positions keep their M1-snapshotted dates (term, dissolution delay) under M2 rules.
- Why accepted: same Option B decision (2026-09-24); the M1 vault is a minimal stopgap and M2 is where redemption, renewal, and the fee-financing upgrade land. Log: `m1-guide.md` section 10.
- Bound on impact: the M2 migration is planned, not implemented or guaranteed by M1. Dates alone do not migrate or close positions. A Closing wallet can become eligible for stranding after `dissolutionEligibleAt`.
- Monitoring: none on-chain in M1.
- Revisit trigger: M2 (this is the M2 scope itself).
- Status: `design-only` (planned, not in code; M2).

## AR-05 Compensation for a stranded position is off-chain only

- Description: when a wallet dies and a reservation strands, the owner keeps fungible TBTC and the in-kind option is lost; the shortfall is shared by all TBTC holders. M1 ships no on-chain compensation: `ReservationStranded` is the record (Tier 0); Tier 1 fee restitution is an off-chain arrangement and out of protocol; the pooled-liquidity risk of stranded positions is borne by the operator until M2.
- Why accepted: the emergency-exit decision (2026-08-21) rejected building a compensation or exit mechanism on unmeasured loss frequency and value; Tiers 0-1 are the only buildable piece and are held as reference. The 2026-10-29 product decision confirms: no refund code, the event log is the record, Tier 1 is off-chain and bespoke. Log: `exit/README.md`; `m1-integrator-due-diligence.md` D6 row ("No compensation exists in M1").
- Bound on impact: stranding does not burn outstanding TBTC or transfer an anchor to its owner. If Bitcoin is lost, the backing shortfall affects pooled TBTC holders. A position marked Stranded does not, by itself, prove physical loss of the output.
- Monitoring: count of `ReservationStranded` events and the `anchorAmount` in each (the evidence payload, `tbtc-v2:solidity/contracts/bridge/Reservation.sol:372`); this is also the data that decides whether the exit mechanism is ever built.
- Revisit trigger: M2 (Tier 0/1 accounting), or earlier if stranding-frequency and anchor-value evidence shows expected annual loss exceeds the mechanism's carrying cost (`exit/README.md` reopen conditions).
- Status: `design-only` (no on-chain compensation in M1; M2/reference).

## AR-06 Re-anchor is unbounded in time; the in-kind claim can grind down

- Description: M1 re-anchor has no position-age limit, so an anchor can be re-anchored at any term age. Each hop may spend up to `reservationTxMaxFee` and the claim shrinks by that hop's fee (`mintedAmount = newAnchorAmount`, `tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:792`); `cumulativeReanchorFee` accumulates the lifetime total but no per-hop or total ceiling is enforced. The only floor is the request-time check that the anchor stays above `txMaxFee + minAmount` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:854`), which caps the grind at `(initial anchor - minAmount)`.
- Why accepted: variant B removed dissolution, so a position past eligibility must still be movable; the time limit was dropped on purpose. Log: `m1-guide.md` section 6 ("Re-anchor therefore had to lose its time limit") and section 4.5 floor note.
- Bound on impact: the in-kind claim can be ground down to just above `minAmount` across many hops; each hop's fee is financed in kind by the vault or becomes debt (AR-09, DR-09). No loss beyond the miner fees actually paid.
- Monitoring: `cumulativeReanchorFee` on the position and the per-hop `ReservationReanchored.minerFee` event; a cumulative-fee limit is a known un-built item (`m1-guide.md` section 9).
- Revisit trigger: M2 (decide whether to reintroduce a time-based eligibility gate and a cumulative-fee cap when dissolution ships; the storage field `reanchorCooldownUntil` and `cumulativeReanchorFee` are already written for it, `tbtc-v2:solidity/contracts/bridge/Reservation.sol:238`, `:241`).
- Status: `holds` (the floor is in code and unit-tested, `tbtc-v2:solidity/test/bridge/Bridge.ReservationSettlement.test.ts:500`; the "no cap" fact is the absence of a ceiling, which is the intended behavior).

## AR-07 `closeReservation` is unreachable in M1

- Description: `closeReservation` exists but has no caller in M1; it is written only for the two M2 settlement call sites (reserved redemption, dissolution). No M1 path marks a position `Closed`.
- Why accepted: closing a position soundly requires moving and proving Bitcoin, which M1 (variant B) does not do; `Closed` is reserved for M2. Log: the function's own doc comment ("Intended for milestone 2 settlement call sites (both currently unreachable in m1)").
- Bound on impact: none by itself; it is dead code until M2. It confirms there is no owner-initiated close (AR-02). Its guard accepts a position in `ActionPending` as well as `Active` (`tbtc-v2@c8350d109:solidity/contracts/bridge/Reservation.sol:1253-1257`) and releases through the same three calls a strand uses (`:1258-1277`), so a caller wired in M2 would be able to close a position whose acceptance or re-anchor generation can still settle and release its wallet and global capacity early.
- Monitoring: none.
- Revisit trigger: M2 (wire the two call sites, and narrow the guard to `Active` or record why a position with a live generation may be closed).
- Status: `holds` (no caller in either repo; the doc and the enum value `Closed` are present, `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1137`, `:100`).

## AR-08 Permissionless third-party re-anchor griefing (beyond D19's slot reuse)

- Description: a third party may request a re-anchor off any MovingFunds or Closing source to any Live target, up to `maxThirdPartyReanchorHoldsPerWallet` holds per target, with no signature required to occupy the target's capacity; only completion needs the source keyset to sign. A single caller can pipeline holds against one Live target within a block, occupying it up to the hold cap and the ordinary per-wallet cap. At `806b1106f` the hold cap still applies to every non-privileged caller (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:896-906`, "Target third-party re-anchor holds exceeded" on `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:901`), but the occupied target no longer blocks a privileged hand-off from a retiring source (AR-01).
- Why accepted: the permissionless re-anchor rule (spec) was kept over caller restriction after the restriction draft conflicted with the "anyone may re-anchor" requirement; the mitigations are the hold cap and the action timeout. Log: `gap-inventory.md` #190 (2026-10-08); `RESERVATION_CAPS_DEPLOYMENT.md` "Accepted Risk Rationale" (occupation is signature-free, bounded only by `reservationActionTimeout`).
- Bound on impact: timeout eligibility starts after one action duration, but cleanup must be mined. A hold can persist longer during caller or chain failure. It reserves accounting, not Bitcoin by itself. At `9d2086965` a pinned target could stall a retiring source's hand-off. At `806b1106f` the retiring-source hand-off is bounded for privileged callers (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:553`). The griefing impact that remains is on third-party requests and on governance rotation off a Live source, which keep both target caps (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:485`). A pending request on the same reservation key still blocks a new request on that key until it completes or times out ("Reservation is not active", `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:783`).
- Monitoring: `pendingThirdPartyReanchorHolds(wallet)`; `spv_reservation_action_overdue_total` for holds that never complete.
- Revisit trigger: D31 is integrated at `806b1106f` (see AR-01 and the approved design-delta chapter). Revisit before mainnet if the Sepolia drill shows a hold pinning a target for third-party requests. Not deployed on Sepolia or mainnet.
- Status: `holds` (the hold cap, the `thirdPartyHold` flag, and the release-on-timeout are in code and unit-tested, `tbtc-v2:solidity/contracts/bridge/Reservation.sol:866`, `test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:133`, `:250` at `9d2086965`; at `806b1106f` the privileged-exempt path is pinned by `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:553`).

## AR-09 Closing-wallet lifecycle has no production trigger in keep-core

- Description: keep-core does re-anchor off a `Closing` source (the re-anchor task accepts `StateClosing`, `keep-core:pkg/tbtcpg/reservation_reanchor.go:170`), and the stranding watcher handles a Closing wallet once it is past `dissolutionEligibleAt` (`keep-core:pkg/maintainer/spv/reservation_stranding_watch.go:139`). But no keep-core component calls `notifyWalletClosingPeriodElapsed`, the permissionless finalize that moves a Closing wallet to Closed once `closingStartedAt + walletClosingPeriod` has passed (`tbtc-v2:solidity/contracts/bridge/Wallets.sol:363`). The Closing->Closed finalization is not automated in the client.
- Why accepted: in M1 a Closing wallet can still be re-anchored off (unlike a Closed/Terminated one), so the absence of the finalize trigger does not strand the reservation by itself; it is recorded as a gap, not a drill failure. Log: `testnet-drill-runbook.md` R2 ("Do not count it as a drill failure. Record it.").
- Bound on impact: a Closing wallet that keeps holding reservations and whose period elapses can sit in Closing indefinitely with no client finalizing it; its reservations remain movable (re-anchor) or strandable (past `dissolutionEligibleAt`). No funds are lost; it is a wallet-retirement liveness gap.
- Monitoring: a wallet stuck in Closing with a non-zero `walletReservationsCount`; `reanchor_waiting_on_aged_target` if re-anchor is blocked on target aging.
  D28 also produces a Critical alert when an Active Closing-source position has no eligible privileged hand-off target
  (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:171-203`,
  `tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:165-172`).
  It does not add a Closing finalizer.
- Revisit trigger: before mainnet, or M2 (add a `notifyWalletClosingPeriodElapsed` caller, or make the stranding watcher re-check Closing wallets periodically; the periodic re-check is partly present for the dissolution-eligible case, `keep-core:pkg/maintainer/spv/reservation_stranding_watch.go:277`).
- Status: `holds-untested` (the Closing branch of the re-anchor task and the Closing-eligible stranding check are in code and have unit tests, `keep-core:pkg/tbtcpg/reservation_reanchor_custody_test.go`, `keep-core:pkg/maintainer/spv/reservation_stranding_watch_test.go`; the missing finalize trigger is the recorded gap).

## AR-10 The late-acceptance window can be missed if the SPV maintainer is down

- Description: a `TimedOut` acceptance can settle late only through
  `timeoutAt + termSeconds` (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:192-210`).
  If its Bitcoin transaction already spent the funding output,
  the depositor cannot use the refund script after that proof window closes.
  The result can be a confirmed anchor with no credited reservation.
- Why accepted: the late window bounds settlement of timed-out acceptance generations
  (TM-16 records the same cutoff as its no-loss limit, VF-13).
  This limit does not apply to a still-Pending action, and late re-anchor has no equivalent time bound.
- Bound on impact: the affected confirmed anchor can remain uncredited.
  No ordinary M1 call reopens an expired TimedOut acceptance.
  This is not a refund or a no-loss outcome.
- Monitoring: retain the original overdue and proof-loop signals. D28 adds a dedicated count near the snapshotted late-proof edge (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:85-106`) and a Critical event (`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:157-164`). Observation-health alerts prevent stale zeros from being treated as healthy. Local acceptance is recorded in [approved design deltas](13-approved-design-deltas.md); live rollout remains pending.
- Revisit trigger: before mainnet, prove that authorized proof submission survives an outage across the cutoff. D28's implemented independent keeper and alert do not close that DR-10 recovery gap.
- Status: `holds` (the window is in code; unit tests on the tbtc-v2 reservations branch pin the inclusive bound, the revert one second later and the unbounded re-anchor: `tbtc-v2@998451722:solidity/test/bridge/ReservationProofs.test.ts:437`, `:449`, `:462`). The outage across the window is not rehearsed (DR-10).

## AR-11 Expiry does not automatically release an accepted position

The published code and the intended policy agree: a position on a healthy (Live)
wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was
taken on 2026-10-29 and the code does not change.

- Description: no transaction runs automatically at `expiresAt` or
  `dissolutionEligibleAt`. The latter date makes an `Active` position on a
  `Live` wallet eligible for explicit stranding, and on a `Closing` wallet it
  is one of two conditions: stranding also waits for
  `closingStartedAt + walletClosingPeriod`
  (`tbtc-v2@c8350d109:solidity/contracts/bridge/Reservation.sol:1325-1336`).
  A `MovingFunds` source does not become strandable solely because time
  passed; a `Terminated` or `Closed` wallet needs no deadline at all.
  The earlier wording denied the Live path, and that denial does not match the published target (VF-10).
- Why accepted: M1 has no owner exit, renewal or dissolution entry point.
  The recorded M2 migration decision does not impose an automatic M1 expiry action.
- Bound on impact: accepted occupancy can outlive the declared term.
  A reservation may lose its in-kind status when eligible stranding is actually submitted.
- Monitoring: compare dates with wallet state and pending actions, not the M2 release date alone.
- Revisit trigger: before mainnet, confirm the promised custody semantics and the M2 migration terms.
- Status: `holds` at `tbtc-v2@c8350d109` for the explicit guards
  (`tbtc-v2@c8350d109:solidity/contracts/bridge/Reservation.sol:1308-1340`).
  The client side is uneven: since keep-core #4365 the watcher notifies for all
  four admissible states (`keep-core@4ced1c839:pkg/maintainer/spv/reservation_stranding_watch.go:155-176`),
  but the `Live` branch runs only on the startup catch-up scan and on the
  action-timeout recheck queue, so a Live-wallet strand that neither path reaches
  still needs another caller.
  Range tests: `tbtc-v2@c8350d109:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:263`
  (Closing past both), `:386` (Live before eligibility), `:397` (Live past it).

## AR-12 Fee debt can grow while the reserve target is zero

- Description: on Sepolia the vault's `feeReserveTarget` is 0 (decision D21), so `sweepFees` can move the vault's entire TBTC balance to the treasury and leave nothing to finance in-kind miner fees; every shortfall is recorded as `inKindFeeDebtSat` and the system runs over-supplied by that amount until repaid.
- Why accepted: D21 chose 0 as the Sepolia value so fee revenue sweeps cleanly during the drill; the debt sink is permissionless and public. Log: `testnet-readiness-plan.md` D21 (2026-10-10); `RESERVATION_CAPS_DEPLOYMENT.md` ("a zero reserve target means sweepFees may sweep the vault's entire TBTC balance ... the shortfall is recorded as inKindFeeDebtSat").
- Bound on impact: total TBTC supply exceeds the Bitcoin backing by exactly `inKindFeeDebtSat`, visible on-chain and repayable by anyone. It is a solvency gap, not a loss of a specific user's sats.
- Monitoring: retain the original reserve gauge. D28 reads debt after every complete observation and sends Warning for any positive value, not only growth (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:170-187`, `tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:173-180`). The local fork debt/repayment control is recorded in DR-09. Repayment remains a human-approved incident action.
- Revisit trigger: before mainnet (set a non-zero `feeReserveTarget` so the reserve survives fee events; mainnet sizing is a governance decision).
- Status: `holds` (the debt accounting and `repayInKindFeeDebt` are in code and unit-tested, `tbtc-v2:solidity/test/vault/ReservationVault.test.ts:273`, `:449`).

## AR-13 The stranding startup scan only looks back 30 days

- Description: when a keep-core maintainer starts, its stranding catch-up scan walks only the last `reservationDefaultLookBackBlocks` (216000 blocks, about 30 days) of wallet registrations. A wallet that closed or terminated more than that bound before the process started, with all maintainers down across the close, has no path to a stranding notification; the code logs a warning when the bound truncates the window.
- Why accepted: the bound keeps startup gas bounded; the risk is a small, bounded set of stale stranding notifications, not funds. Log: the wiring comment ("an accepted operational limitation, not a gap covered elsewhere").
- Bound on impact: a stranded position that is never notified keeps occupying a slot and a global count until it is notified later or its wallet re-appears; it does not create a new loss, it delays the evidence and the capacity release.
- Monitoring: the warning log when the startup scan bound truncates; a manually discovered unnotified `ReservationStranded` candidate.
- Revisit trigger: before mainnet (raise the lookback or run the startup scan from the reservation activation block, which the wiring already computes, `keep-core:pkg/maintainer/spv/reservation_wiring.go:440`).
- Status: `holds-untested` (the bounded scan and the truncation warning are in code, `keep-core:pkg/maintainer/spv/reservation_wiring.go:136`, `:742`; not rehearsed).

## AR-14 A re-anchor timeout starts a cooldown that blocks everyone but governance

- Description: when a re-anchor generation times out, the position returns to Active on the source wallet and a cooldown equal to the action's signing window is set (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:984`). During it, only a governance call may request a new re-anchor; the owner, SPV maintainers, source-wallet members, and third parties all wait (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:758`).
- Why accepted: the cooldown prevents a tight request-timeout-request loop from keeping a retiring wallet's slot churning; it is the price of the permissionless re-anchor backstop. A late proof of the timed-out generation is still accepted, so the cooldown does not forfeit a confirmed tx.
- Bound on impact: a retiring wallet may wait up to one signing window plus the next governance ceremony before a fresh re-anchor can be requested. No funds move; it is a delay on the migration.
- Monitoring: `reanchorCooldownUntil` on the position; `spv_reservation_action_overdue_total`.
- Revisit trigger: M2 (the cooldown and the unbounded re-anchor are the two re-anchor policy questions M2 must settle, `m1-guide.md` section 10).
- Status: `holds` (the cooldown write and the privileged-only exemption are in code, `tbtc-v2:solidity/contracts/bridge/Reservation.sol:758`, `:984`).

## AR-15 `activeReservationsCount` under-reports per-wallet slot pressure during pending re-anchors

- Description: while a re-anchor is pending, the global `activeReservationsCount` does not move (re-anchor only changes the target's per-wallet count at request time and the source's at settlement), so an occupancy alert built on `activeReservationsCount` can under-report the per-wallet slot pressure that a keep-core free-slot monitor would see. At `maxReservationsPerWallet = 1` this is the exact case where one pending re-anchor holds both the source and target slots.
- Why accepted: documented operational caveat in the deploy runbook, not a code change; the alert is advisory. Log: `RESERVATION_CAPS_DEPLOYMENT.md` "Occupancy-Count Divergence".
- Bound on impact: an operator may believe there is free per-wallet capacity when there is not, and a re-anchor may revert with "Wallet reservations cap exceeded" that the global gauge did not predict. At `806b1106f` a privileged hand-off from a non-Live source skips that revert (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`); a third-party or Live-source governance request still hits it. The new acceptance reserve reads the same global count (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1071-1077`), which a re-anchor does not change, so the divergence is unchanged.
- Monitoring: track per-wallet `walletReservationsCount` and `pendingThirdPartyReanchorHolds` alongside the global count. The drill runbook reads only `activeReservationsCount()` and `liveWalletsCount()` before each step (`testnet-drill-runbook.md:35`); it does not read the two per-wallet values.
- Revisit trigger: before mainnet (make the occupancy alert per-wallet-aware when sizing the mainnet `maxActiveReservations`).
- Status: `holds-untested` (the divergence is a property of the request-time reservation, in code; not a defect, just a monitoring gap).

## AR-16 The `receiveBalanceIncrease` ordering gap is enforced by the deploy script, not on-chain

- Description: the reservation vault's `receiveBalanceIncrease` is `onlyBank` and trusts every Bank-routed credit with no `reservationKey` parameter, so it cannot verify that a credit corresponds to a Bridge-proven reservation anchor (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:147`). The code comment records this as a known gap: the deploy pipeline marks the vault `isVaultTrusted` only after it is wired into the Bridge on non-local networks, so the scripted activation path cannot trigger the gap. But that ordering is enforced by the deploy script, not by an on-chain check, so governance retains the raw ability to call `setVaultStatus(vault, true)` directly through `BridgeGovernance` and bypass the script's precondition.
- Acceptance status: accepted for the Sepolia drill only (human decision D32, 2026-10-12). The control is the scripted order: script 97 wires the vault into the Bridge before `setVaultStatus(vault, true)`, and the drill runbook checks the wiring before the trust call. An on-chain guard must land in the PR before mainnet. The source comment names two candidate remedies, a Bridge-only credit entry point or an ordinary-sweep guard in `DepositSweep` (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:157-160`); neither is selected yet.
- Bound on impact: a trusted miswired vault can handle ordinary pooled credit through the reservation fee/mint path. The demonstrated credit remains backed by pooled BTC and consumes Bank balance. VF-02 rejected arbitrary over-issuance.
- Monitoring: the deployment log must show script 97 wiring the vault before it trusts it; a direct `setVaultStatus` that precedes the wiring is the failure signature. Before the trust call, the drill runbook reads `reservationParameters()` through the Bridge and confirms that its first value is the deployed vault (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:585-589`). There is no on-chain monitor because the gap is the absence of a check.
- Revisit trigger: before mainnet, select and land the on-chain guard in the PR (D32). A governance plan to trust a second reservation vault, or to make a past release destination the reservation vault (VF-09), needs a new review first.
- Status: `holds-untested` (the unconditional credit path is in code, `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:147-195`; no test drives ordinary credit through a trusted vault that is not wired; accepted for Sepolia only, D32).

## AR-17 An anchor output above the action amount reverts with an arithmetic panic

- Description: the proof-time fee check is written as `action.amount - anchorAmount <= action.txMaxFee` (`tbtc-v2@c8350d109:solidity/contracts/bridge/ReservationProofs.sol:462-465`). An anchor output that pays the wallet more than the snapshotted `action.amount` underflows that subtraction, so the call reverts with the arithmetic panic `0x11` instead of the `Transaction fee is too high` message the same check returns in every other case.
- Why accepted: it needs a proof of a Bitcoin transaction that pays the wallet more than the depositor reserved, which the wallet does not sign, and no test in the suite drives the branch. The fix is a contracts edit (bound `anchorAmount` before subtracting, or compare in the other direction) and this review set carries no code changes.
- Bound on impact: a rejected proof with a misleading reason. The anchor cannot move and no state changes, so no value is at risk.
- Monitoring: none. The condition would show only as a failed acceptance proof.
- Revisit trigger: with the next validated change to the proof validators, where the bound and its test land together.
- Status: `holds` (the require and its message are at the cited lines at the final head; the branch above `action.amount` is not exercised).

## AR-18 `feeReserveTarget` is a sweep threshold, not a floor

- Description: `financeInKindFee` finances the miner fee of a settled re-anchor out of the vault's whole TBTC balance, and `_burnFromReserve` scales the burn down only to that balance, not to the target (`tbtc-v2@c8350d109:solidity/contracts/vault/ReservationVault.sol:357-360`). `feeReserveTarget` is read by `sweepFees` alone, as the amount left behind when the excess is swept (`:310-316`). A settlement can therefore spend fee revenue that governance meant to keep, and the balance can end below the target.
- Why accepted: deliberate and stated in the contract's own doc comment, "by feeReserveTarget, which only gates what sweepFees can remove" (`tbtc-v2@c8350d109:solidity/contracts/vault/ReservationVault.sol:198`): a confirmed Bitcoin spend must settle rather than fail on the reserve level, and the shortfall it creates is recorded as debt (AR-12). Making the target a floor is a contracts edit inside a reviewed PR.
- Bound on impact: revenue earmarked for sweeping is spent on in-kind miner fees; the balance cannot go below zero and the uncovered part becomes `inKindFeeDebtSat` (AR-12). Any external accounting or dashboard that reads the target as a floor will disagree with the contract.
- Monitoring: `feeReserveTarget()` against the vault's TBTC balance, and `inKindFeeDebtSat` after a settlement that outran the reserve.
- Revisit trigger: before mainnet, with the reserve sizing decision (AR-12). Either state in the deployment documentation that the target is a sweep threshold, or make it a floor.
- Status: `holds` (both readers are at the cited lines at the final head; the contract's doc comment states the target does not bound this path).

---

## Doc drift

| Doc path:line | What it says | What the code does |
|---|---|---|
| `testnet-drill-runbook.md:75` (R2) | "A `Closing` wallet has no automated re-anchor or stranding trigger in M1." | The keep-core re-anchor task does re-anchor off `Closing` (`keep-core:pkg/tbtcpg/reservation_reanchor.go:170`) and the stranding watcher handles a dissolution-eligible Closing wallet (`keep-core:pkg/maintainer/spv/reservation_stranding_watch.go:139`). What has no trigger is the Closing->Closed finalization (`notifyWalletClosingPeriodElapsed`), AR-09. |
| `m1-guide.md:75` | "A late re-anchor proof has no time limit while the source anchor is still current." | Confirmed in code: the unbounded late re-anchor is at `tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:700` and the only time-bound late path is acceptance (`:209`). Recorded so the two are not read as symmetric; the re-anchor is unbounded, the acceptance is not (AR-10). |
| `m1-guide.md:213` | The Sepolia `feeReserveTarget` table entry is `0 (D21)`. | The vault constructor sets `initiationFeeBps` to 40 but leaves `feeReserveTarget` at its zero value and the Sepolia activation script does not call `updateFeeReserveTarget` (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:78`, `:135`; deploy script `97_set_reservation_parameters.ts` performs only the caps and parameter updates). So 0 is a deployment fact, not a code default a reader can see in the constructor. |

---

## Findings

No AR entry is `violated`; every entry is an accepted limitation or a recorded gap with a code reference. The two entries most likely to become defects if left unrevisited before mainnet are AR-10 (the late-acceptance window is unit-tested, but an SPV outage across it is not rehearsed) and AR-09 (the missing Closing->Closed finalize trigger); both have a revisit trigger set before mainnet.

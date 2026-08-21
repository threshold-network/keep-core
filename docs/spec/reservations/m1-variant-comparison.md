# m1 Variant Comparison — A+ vs B

Status: DRAFT — decision support, 2026-08-21.

Side-by-side comparison of the two candidate designs for a from-scratch,
essentials-only m1. Both are rewrites rather than PR omissions, so neither
deploys the 445 unreachable lines the stacked design carries (`roadmap.md`
§5.1). `roadmap.md` §5.2 holds the derivation and the full rung ladder
(0, 0+, A, A+, B); this doc is the flat comparison of the two rungs that are
actually in contention.

**They differ in exactly one feature: dissolution.**

---

## 1. What both include

| # | Feature | Why it cannot be cut |
|---|---|---|
| 1 | Deposit routing + reveal-time classification (`revealDepositWithReservation`, `registerReservation`, `pendingReservedDeposits`) | The entry gate; without it nothing is a reservation |
| 2 | Two-phase acceptance (`requestReservationAcceptance` -> `submitReservationAcceptanceProof` -> `settleAcceptance`) + designated-wallet binding (M-05) | Load-bearing safety: a Byzantine wallet cannot force custody onto an unrelated Live wallet |
| 3 | Mint + backing invariant (`mintedAmount == anchorAmount`) | The correctness model every action obeys |
| 4 | **Re-anchor, unbounded** (rotation/migration: request + proof) | Wallet health: a wallet holding anchors cannot begin closing until its reservation count reaches zero |
| 5 | Action timeout + `unwindPendingAction` | Otherwise a failed action leaks capacity permanently |
| 6 | Stale deposit cleanup (`notifyStaleReservedDeposit`) | Un-accepted revealed deposits must be releasable |
| 7 | Stranding (`notifyReservationStranded` / `strandReservation`, requires wallet `Terminated`) | Dead-wallet capacity release |
| 8 | Caps + governance parameters (`updateReservationParameters`) | The only safety valve at launch |
| 9 | Wallet-lifecycle guards, full storage layout | §11's no-live-action-migration rule (`feature-spec.md` §3.4) |
| 10 | Router, **only if** EIP-170 requires it | Deleting it saves ~736 prod / ~2,000 with tests off either variant — a smaller swing than the dissolution choice below (910 / ~2,500). The stacked router file is 1,051 prod / ~3,400 with tests; a rewritten one is leaner. Decided by the compiler, not by argument |

**Absent from both** (deferred to m2): in-kind redemption whole and partial,
renewal / `extendCustody`, redemption veto and watchtower integration, retry
credit, renewal pause machinery. Both variants are create-only — no reachable
in-kind exit exists in m1 (`roadmap.md` §0.2).

## 2. The one difference

| Feature | A+ | B |
|---|---|---|
| Dissolution (`requestReservationDissolution`, `settleDissolution`, `supersedeConflictingDissolution`, `processDissolutionInputs`, `validateDissolutionOutput`) | **present** | **absent** |

~310 function code-lines, ~910 shipped lines including storage, events,
natspec and tests.

Note the ordering constraint from `roadmap.md` §5.2: dropping dissolution is
only sound *because* re-anchor is unbounded in both variants. With
dissolution absent and re-anchor still gated at `< dissolutionEligibleAt`, a
position past its eligibility date would have no unpin path at all.

## 3. Lines of code

| | A+ with router | A+ no router | B with router | B no router |
|---|---|---|---|---|
| Production Solidity | 6,171 | 5,435 | 5,261 | 4,525 |
| + tests (1.7-1.9x) | 16,700-17,900 | 14,700-15,800 | 14,200-15,300 | 12,200-13,100 |
| vs stacked m1 (9,206) | -33% | -41% | -43% | -51% |
| keep-core production Go | ~1,400-1,900 | same | ~1,100-1,400 | same |

**A+ -> B saves 910 production Solidity (-15%), ~2,500 lines including tests,
and ~300-500 production Go.** The router question is the cheaper of the two
to resolve — a compiler run rather than a protocol decision — but it is the
smaller swing (736 / ~2,000).

## 4. A+ — pros and cons

**Pros**

- **Two exits; pinning solved unconditionally.** Re-anchor works at any
  position age, and dissolution opens post-eligibility.
- **Capacity is self-renewing.** Positions close, so wallet slots recycle and
  both enforced caps stay concurrent. No operational duty.
- `maxReservationsPerWallet` can stay at **1** — the tightest pinning blast
  radius — with no capacity pressure.
- `reservationTotalAmount` tracks live exposure, so cap monitoring means what
  it says.
- **Superset of B**, so m2 never has to add dissolution back.

**Cons**

- **Keeps the §0.6 slashing vector.** `requestReservationDissolution` is
  permissionless (`ReservationRouter.sol:302-304` has no modifier;
  `Reservation.sol:887-890` has no `msg.sender` check) and
  `notifyReservationActionTimeout` slashes the wallet operators on
  non-execution, exactly like a pooled redemption timeout
  (`Reservation.sol:961-975`). An unwired keep-core hands a passer-by the
  ability to slash honest wallets.
- **keep-core dissolution support is mandatory and sits on the serial
  pre-audit path** (~300-500 production Go plus tests).
- 910 more production Solidity lines to audit.
- The 12-month term **dates a hard deadline** for that keep-core work.

## 5. B — pros and cons

**Pros**

- **Slashing vector removed.** No dissolution entry point exists, so there is
  nothing to leave unwired.
- **Schedule saving, not only a code saving.** The mandatory keep-core duty
  leaves the critical path — the axis that actually sets the launch date
  (`timeline-estimate.md` §6).
- **Smallest audit surface** of any rung: 4,525-5,261 production Solidity.
- No dated §0.6 deadline.

**Cons**

- **Nothing closes a position except wallet termination.** Wallet slots are
  never freed and the global amount cap becomes cumulative-ever rather than
  concurrent.
- **Pinning fix is conditional on free slots** — and §5.3 traces where that
  condition ends. Total slots are
  `walletCount x maxReservationsPerWallet`. Re-anchor requires
  `targetCount <= maxReservationsPerWallet` (`Reservation.sol:820-827`), so a
  hop needs a Live target holding zero anchors, and the source slot is
  released only at settlement (`:819`) — an in-flight hop holds two.
- The governance knob is real but **multiplies the budget without ever
  refilling it**: the count cap carries no validation bound
  (`Reservation.sol:1239-1281`) and applies live to existing wallets (`:1281`,
  read at `:824`), and the m1 launch value of 1 sits a factor of ten below the
  documented default (`feature-spec.md` §10).
- Raising the cap trades directly against the pinning blast radius that
  `roadmap.md` §1.4 set the cap to 1 in order to bound.
- **Standing operational duty**: watch slot occupancy, turn two dials.
- **m2 must build dissolution from scratch** — unlike redemption, where
  `#1096` is already written (`roadmap.md` §5.1).
- Does not scale past design-partner volumes.

## 5.1 What m2 costs after a rewrite

A rewrite does not only shrink m1; it **moves mass into m2**, because the
redemption and renewal code A+ and B cut is code the stacked plan already has
written and reviewed. Taking A+ with router as the m1:

| | Production Solidity | + tests (1.7-1.9x) | m1 : m2 |
|---|---|---|---|
| m1 (A+ with router) | 6,171 | ~16,700-17,900 | — |
| m2 after A+ | **3,731** | ~10,100-10,800 | **1.65 : 1** |
| *(stacked plan for contrast)* | *m1 9,206 / m2 696* | *27,041 / 3,259* | *13.2 : 1* |

m2's 3,731 is 3,035 lines of whole redemption, renewal, watchtower veto
integration and their storage — the exact material A+ removed — plus `#1096`'s
696 lines of partial redemption.

Two consequences:

- **Cumulative mass is unchanged — by assumption.** 6,171 + 3,731 = 9,902
  production lines, identical to the stacked path's 9,206 + 696. That
  equality is arithmetic, not measurement: m2's restore figure is taken as
  exactly the 3,035 lines A+ removed, i.e. **the re-added redemption and
  renewal are assumed to be written at stacked density**, not at the leaner
  keep-factor density applied to everything A+ retained. Held to the same
  discipline the retained files got (~0.7), the restore is nearer ~2,100 and
  the cumulative total nearer ~9,000 — a real but modest saving. Either way
  the rewrite mainly reallocates the audit between milestones rather than
  reducing it.
- **m2 stops being cheap.** Under the stacked plan m2 writes **zero** new
  Solidity — `#1096` is an open PR with its 3,259 lines already written
  (`roadmap.md` §5.1) — so the only new work is ~600-1,100 lines of keep-core
  Go. After a rewrite, m2 must write ~3,035 production lines of Solidity from
  scratch, with `#1096`'s work only partly adaptable since it targets the
  stacked structure.

So the rewrite's -33% m1 audit is financed by a ~5.4x larger m2 audit and a
second body of Solidity to write. That is the same conclusion §6 reaches from
the volume side, arrived at from the code side.

## 5.2 Launching B with no router

Attractive on paper — 4,525 production lines, -51% against the stacked m1,
the smallest surface on the ladder. Four drawbacks; the second is the one
that actually bites.

**1. Whether it fits is undetermined — one compiler run settles it.** Do not
price the inline cost at the router's standalone size. `feature-spec.md` §2
reports three measurements, and two of them bracket the answer by
subtraction:

| Measurement | Bytes |
|---|---|
| EIP-170 limit | 24,576 |
| `Bridge` with the surface inline, pre-`#1090` | 24,647 (over by **71**) |
| `Bridge` after moving it out (`runs=100`) | 22,403 (margin **2,173**) |
| **Inline cost of the surface, by subtraction** | **~2,244** |
| `ReservationRouter` standalone | 4,245 |

The surface costs `Bridge` ~2,244 B inline, **not** 4,245 B. The router is
larger standalone because it carries its own selector dispatcher plus a second
copy of boilerplate `Bridge` already has — both declare
`Governable, Initializable`, and §2 invariant 2 explicitly exempts "Governable
members shared by both" from the shadowing test. The bracket is
self-consistent: 2,244 against 2,173 of margin is over by exactly the 71 B
that was measured pre-`#1090`.

So the gap to close is **71 B, not 2,072 B**, and dropping 5 of 24 entry
points (`requestReservedRedemption`, `notifyReservedRedemptionVeto`,
`extendReservation`, `requestReservationDissolution`,
`walletPendingDissolution`) plus simplifying `submitReservationProof` clears
71 B comfortably — one external wrapper with calldata decoding exceeds that
alone.

The genuine unknown is growth: all three figures are `#1090`-era, and
`#1091`-`#1096` each added entry points and logic, so today's inline cost sits
somewhere between 2,244 B and 4,245 B and is unmeasured. Verdict:
**undetermined, and cheap to determine** — compile B's surface into `Bridge`
at `runs=100` and read the size. This is not a reason to reject
B-no-router on the evidence available.

**2. It removes the extension budget precisely where it is guaranteed to be
needed.** B is by construction a launch posture that m2 must replace (§5), and
§5.1 shows m2 has to add ~3,035 production lines of whole redemption, renewal
and veto integration plus `#1096`'s partial redemption — ten-plus new entry
points. With no router there is nowhere to put them. Note this argument does
**not** depend on drawback 1's open question: even if B's reduced surface fits
`Bridge` today, m2's additions are larger than the surface B removed, so they
cannot. B-no-router therefore **defers** the router rather than deleting it,
the same shape as its capacity loan.

**3. Introducing the router later costs a `Bridge` implementation upgrade.**
The fallback dispatcher and the one-time `setReservationRouter` live in
`Bridge.sol` (`:2114-2148`). Shipping without them makes m2's router a
proxy-admin ceremony. **Cheap mitigation:** ship the fallback and the setter
in m1 with the router address left unset — `fallback` then reverts
`"Unknown function"` harmlessly, and the option costs a few hundred bytes.
Worth doing even if the surface does fit.

**4. B is the variant least able to afford no headroom.** It carries a
standing slot-occupancy duty and no dissolution path, so the plausible hotfix
is exactly the kind that needs new bytecode — a governance force-close to
unpin a wallet, say. With the router there is ~20 kB free for it; without,
there is none. Note the limit of this argument: the router buys **space, not
ceremony** — invariant 4 means replacing router code is a `Bridge`
implementation upgrade either way.

A fifth, softer point: the router question is compiler-decidable and
reversible, while B is a protocol decision. Bundling them means a compiler
surprise re-opens the launch shape.

**Verdict: take B with the router if B is taken at all** — but on drawback 2,
not on drawback 1. Whether m1's reduced surface fits `Bridge` is unmeasured
and may well be fine; what is not fine is that m2 provably needs the router
back, so skipping it in m1 buys ~736 lines against a re-architecture later.
The router is also the smaller of the two swings (736 lines against
dissolution's 910), and B is the rung most likely to need room to move.

## 5.3 The B endgame, verified

§5's cost list called B's pinning fix "conditional on free slots". Tracing the
chain to its terminal state makes it sharper: the condition does not merely
degrade, it **resolves into slashing and stranding**. Every step is
source-verified on `feat/utxo-reservation-guards`.

1. In B nothing closes a position except stranding. `settleDissolution` is
   gone, `ReservationProofs.sol:715`/`:836` are the redemption path
   (unreachable, §0.2), and `strandReservation` requires the wallet to be
   `Terminated`.
2. Occupancy is therefore **monotonic**, tending to
   `liveWalletsCount x maxReservationsPerWallet`.
3. At full occupancy re-anchor reverts: it needs a Live target holding a free
   slot (`targetCount <= maxReservationsPerWallet`, `Reservation.sol:820-827`).
4. An anchored wallet then cannot retire. `beginWalletClosing` requires
   `walletReservationsCount == 0` — "Wallet still custodies reservations"
   (`Wallets.sol:675-677`, repeated at `:707-709`).
5. Its MovingFunds clock expires and `notifyWalletMovingFundsTimeout`
   **seizes operator stake** (`ecdsaWalletRegistry.seize` with
   `movingFundsTimeoutSlashingAmount`) and terminates the wallet
   (`Wallets.sol:493-523`).
6. Termination makes `notifyReservationStranded` available, converting the
   depositor's segregated in-kind claim into an ordinary pooled claim.

**Step 5 corrected 2026-08-21.** The chain above assumes the MovingFunds clock
is still running. It is not, if the wallet proved its funds moved:
`notifyWalletFundsMoved` deletes `movingFundsRequestedAt` (`Wallets.sol:434`,
"Zero is the completion sentinel and cannot be reported as a timeout") and
`notifyMovingFundsTimeout` requires it non-zero
(`MovingFunds.sol:594-598`). So the endgame has two branches and step 6 does
not always follow:

- **Wallet never proves funds moved:** the chain holds as written. Stake is
  seized, the wallet terminates, and the position becomes strandable.
- **Wallet proves funds moved and keeps anchors:** no slashing at all, but also
  no termination, so `notifyReservationStranded` never becomes available. The
  position has no close path until a slot frees and re-anchor can drain it.

See `roadmap.md` §0.8 for the full derivation, including the permissionless
below-dust exit that closes such a wallet once its count reaches zero.

So B's capacity limit terminates in **stranded depositors, and slashed
operators only on the branch where the wallet never proved its funds moved** —
the same harm class as the §0.6 vector B was chosen to remove, arriving through
a different door. On the other branch operators keep their stake but the
position loses even the stranding exit, which is worse for the depositor.
Either way the conclusion that decides the variant choice is unchanged: §0.6 is
a **liveness duty**, dischargeable by wiring keep-core correctly; this is a
**capacity limit**, dischargeable by no amount of client-side diligence.
And it is reached by arithmetic, not by an attacker.

**The condition that makes B defensible** — the amount cap must bind before
the slot cap ever does:

```
reservationMaxTotalAmount / reservationMinAmount  <<  liveWalletsCount x maxReservationsPerWallet
       (max positions the amount cap permits)              (total slots)
```

At design-partner scale (§1.3: tiny `reservationMaxTotalAmount`, cap = 1) this
holds with wide margin. It fails **silently** if the amount cap is later
raised without slots growing to match — nothing on-chain relates the two
today.

## 5.4 If B ships anyway: the holes to close

**Security, contract-level**

1. **Global active-position cap, enforced at acceptance.** The strongest
   single addition, because it converts the silent cliff into a revert.
   `BridgeState.Storage` has `liveWalletsCount` (`:253`),
   `reservationTotalAmount` (`:378`) and per-wallet counts, but **no global
   position counter** — so add `activeReservationsCount` plus a governance
   `maxActiveReservations`, set conservatively below the slot floor, and
   require it at acceptance. Roughly 20 lines and one parameter; makes §5.3's
   endgame unreachable by construction rather than by monitoring.
2. **Do not add a bookkeeping-only force-close.** Already analysed and
   rejected (`roadmap.md` §0.4): applied to a Live wallet it orphans the
   anchor UTXO outside `mainUtxo` with no authorising spend path while the
   owner keeps their claim, *creating* an `anchorAmount` shortfall instead of
   recognising one. Stranding is sound only because a `Terminated` wallet's
   BTC is already presumed lost. Stated explicitly so it is not re-proposed
   as the hotfix when the cliff approaches.
3. **Storage-complete now, behaviour-minimal now** (§2.1) — and *written*,
   not merely declared. Every field the full feature will need must exist in
   `BridgeState.Storage` at m1, because §11 forbids a live
   `ReservationAction` from spanning a layout change. B has a concrete trap
   here: acceptance writes
   `dissolutionEligibleAt = expiresAt + reservationDissolutionDelay`
   (`ReservationProofs.sol:537-539`), and B's only reader of that field was
   re-anchor's `< dissolutionEligibleAt` gate, which B deletes. An
   essentials-only rewrite would therefore drop the write as dead — after
   which m2's dissolution has **no eligibility date for any m1-era position**,
   and the snapshot semantics that make governance changes non-retroactive
   (`:180-184`) cannot be reconstructed. Keep writing every deferred path's
   fields at m1 even with no reader. This is what makes m2 an upgrade rather
   than a migration.
4. **Relate the two caps in governance.** Nothing today prevents raising
   `reservationMaxTotalAmount` past slot capacity. Either add the relational
   check or emit both sides in `ReservationParametersUpdated` and make it a
   runbook gate — but do not leave §5.3's condition purely tribal.

**Operational**

1. **Re-anchor executor on `WalletMovingFunds`.** In B this is the *only*
   unpin, so executor failure ends in slashing rather than delay. Needs
   alerting, not just logging.
2. **Free-slot monitor** — count Live wallets with
   `walletReservationsCount < maxReservationsPerWallet`. This is the leading
   indicator of §5.3; alert when the target pool falls below a floor.
3. **Occupancy monitor** — `activeReservationsCount` against
   `liveWalletsCount x cap`, alerting well before saturation.
4. **Action-timeout watch** — pending acceptances and re-anchors approaching
   `reservationActionTimeout`, since expiry slashes.
5. **Stranding watcher** on `Terminated` wallets, to release capacity.
6. **Stale reserved-deposit cleanup** (`notifyStaleReservedDeposit`).
7. **Cap-dial runbook** — pre-agreed trigger, executor and the accepted
   blast-radius consequence of each raise, decided before launch rather than
   under pressure.
8. **Position-age report.** Nothing closes in B, so age is the only proxy for
   accumulating permanent liability.

**Making m2 easier**

Note what is *not* available: replacing router code is a `Bridge`
implementation upgrade by design (invariant 4), so no m1 choice avoids that
ceremony. What m1 can do is ensure the ceremony is a code swap and nothing
more — storage complete (item 3), the four router invariant tests pinned so a
successor router inherits them, deferred selectors absent from m1's router but
their storage present, and the m2 sequence written into the release runbook
now while the reasoning is fresh.

## 5.5 If A+ ships: the holes to close

A+ is the recommended path (§6), so it needs its own checklist. Most items
are shared with B — only the first is A+-specific, and it is the one that can
slash honest operators if it slips.

**Launch-blocking**

1. **keep-core dissolution wired before the first position's
   `dissolutionEligibleAt`.** A+-specific and the sharpest item.
   `requestReservationDissolution` has no `msg.sender` check
   (`Reservation.sol:887-890`) and its router wrapper has no modifier
   (`ReservationRouter.sol:302-304`), so **any passer-by can open a
   dissolution against an honest wallet** once a position passes eligibility.
   If no executor submits the proof within `reservationActionTimeout`,
   `notifyReservationActionTimeout` slashes the wallet operators — the natspec
   is explicit that `walletMembersIDs` is "only consulted for redemption and
   dissolution timeouts (the slashing path)" (`:965-967`). So the term length
   is not only a promise clock; it is the deadline by which the dissolution
   executor must be in production. This is the §0.6 duty, and it is
   dischargeable — but only by shipping client code, not by a contract
   parameter.
2. **Every path m2 wants must ship its entry point in m1, flag-gated**
   (shared; `roadmap.md` §0.2/§2.2). This is stronger than "defer the vault
   work", because a path the m1 vault omits **cannot be added later while any
   position lives**: the vault is plain `Ownable`, not proxy-upgradeable, and
   re-pointing `Bridge.reservationVault` requires
   `reservationTotalAmount == 0 && pendingReservedDeposits == 0`
   (`Reservation.sol:1267-1274`).
   - **Renewal.** The shipped vault already does this correctly — entry point
     at `ReservationVault.sol:380-392`, `renewalsPaused = true` in the
     constructor (`:222`), `unpauseRenewals()` `onlyOwner` (`:415-418`). But
     A+ and B **cut renewal**, so on the rewrite path renewal is not
     deferred-to-m2, it is **permanently unreachable** for every m1-era
     position unless the entry point ships paused.
   - **Redemption.** `redeemReservation` ships (`:293`) with **no pause flag**
     — `pauseRenewals`/`blockRenewal` (`:409`, `:424`) are renewal-only. Add
     `redemptionsEnabled`, default false, by copying renewal's pattern one
     function over in the same file.
   The consequence for §1.4's promise clock is the sharp part: if renewal has
   no entry point, the first cohort's in-kind deadline **cannot be bought back
   by extending the term**. The 12 months is then final for those positions.
3. **`reservationsByAnchorUtxo` reconciliation** (shared; `roadmap.md` §4.3
   item 3). `#1091` introduces the mapping (`ReservationProofs.sol:465`) and
   `#1094` writes it again for stranding. **Two write sites, no removal** -
   corrected 2026-08-21. The earlier claim that `#1102` removed it in favour
   of `spentMainUTXOs` was wrong on two counts: the mapping does not exist on
   `#1088`'s branch at all, so `#1102` had nothing to remove, and
   `spentMainUTXOs` is a pre-existing Bridge registry that reservations write
   into (`Reservation.sol:1454`, `:1510`), not a competing index. Reconcile
   the two write sites or stranding breaks, and stranding is the fallback the
   whole loss story rests on (`exit/README.md`).
4. **Storage-complete at m1** (shared, §5.4 item 3). §11 forbids a live
   `ReservationAction` from spanning a layout change, so every field m2 will
   read must exist unread at m1.

**Operational** — the same list as §5.4 minus the free-slot and occupancy
monitors, since dissolution recycles slots, plus one addition: the
**dissolution executor** joins acceptance and re-anchor as a third action type
on the same coordination executor. That is incremental work on a component
that must exist regardless, which is precisely why §6 prefers this duty over
B's cliff.

**What A+ does not need:** the global active-position cap (§5.4 item 1) is
not load-bearing here, because positions close and both caps stay concurrent.
The cap-dial runbook and position-age report also fall away.

## 6. How to choose — and what was chosen

**Decision, 2026-08-21: m1 is variant B with a minimal router.** The analysis
below recommended A+ and is retained unchanged, per this set's convention of
keeping reversed reasoning visible. Read it as the case that was weighed and
the risk register that comes with the choice, not as a live dispute.

What the decision commits the build to, all of it derived from the §5.3-§5.4
analysis below and specified in `m1-b-implementation.md`:

- The §5.4 items stop being conditional. The global active-position cap in
  particular moves from "if B ships" to a **launch gate** — the row below that
  says *reject B without that cap* still holds and is now the gating
  condition, not an argument against the variant.
- §5.5 item 2 turns out to bind harder under B than under A+: because B's
  positions close only by stranding, the vault-swap gate
  (`reservationTotalAmount == 0`) is unreachable while the product is in use,
  so the m1 vault must ship its **full** entry-point surface behind pause
  flags or those paths are closed rather than deferred (`roadmap.md` §0.7).
- The `dissolutionEligibleAt` write trap (§5.4 item 3) becomes a concrete
  build instruction, since B removes the field's only reader.

The one asymmetry the decision accepts knowingly: A+'s cost was a
dischargeable liveness duty, B's is a capacity limit no client-side diligence
discharges. §4.1 of `m1-b-implementation.md` is what bounds it.

### 6.1 The analysis that was weighed (recommended A+)

Earlier drafts framed this as a volume question. §5.3 supersedes that:
B's terminal state is reached by arithmetic, so the question is whether an
on-chain bound keeps the system away from it.

| Condition | Choice | Reason |
|---|---|---|
| Default | **A+** | Keeps dissolution — the only sound unpin for a *live* wallet. B's substitutes are a governance dial and a promise |
| B, only with §5.4 item 1 shipped | acceptable | A global active-position cap below the slot floor makes §5.3 unreachable by construction |
| B without that cap | **reject** | The endgame is stranded depositors, with operators slashed only on the branch where the wallet never proved its funds moved (§5.3, step 5 correction); on the other branch the position loses even the stranding exit. Monitoring cannot prevent either - only notice it |

**Recommendation: implement A+.** Not a confirmed decision — the call is the
team's — but the argument is one-sided enough to state plainly:

- B's saving is 910 production Solidity lines and ~300-500 Go. §5.4 item 1
  spends ~20 of those lines plus a governance parameter just to make B safe,
  and §5.4's operational list is longer than A+'s.
- A+'s cost is the §0.6 permissionless-dissolution slashing vector, which is a
  **liveness duty on an executor that must exist anyway** for acceptance and
  re-anchor. Adding a third action type to a working executor is incremental;
  it is not a new class of work.
- B's cost is a capacity limit whose terminal state is slashing and stranding,
  and no client-side diligence discharges it.
- A+ therefore trades a dischargeable duty for an undischargeable hazard —
  the wrong direction.

A+ dominates rungs 0, 0+ and A outright — every step up to it is free or a
pure omission. Against B it is a judgement, and the judgement is that a
dischargeable duty beats a structural cliff.

---

## Provenance

Derived 2026-08-21 from `roadmap.md` §5.1/§5.2 (measurements and rung
ladder), `feature-spec.md` §10 (governance parameter surface, including the
documented count-cap default of 10) and the following source reads,
branch-tagged because the expiry and guard models differ across the stack:

- `feat/utxo-reservation-settlement` (#1091): re-anchor authorization
  (:718-757) with its pre-`#1093` grace-period gate (:742), close helper
  (:1183-1192), dissolution request payload (:887-892).
- `feat/utxo-reservation-guards` (#1094, on top of `#1093`): re-anchor
  `state == Active` (:781) and `< dissolutionEligibleAt` (:785-788), target
  slot cap (:820-827) with source release only at settlement (:819),
  permissionless dissolution (:887-890) with eligibility gate (:895-901) and
  current-custodian read (:904), timeout slashing (:961-975), parameter
  validation with no bound on the count cap (:1239-1281),
  `notifyReservationStranded` (:1363-1378), router wrapper
  (`ReservationRouter.sol:302-304`), position-closing sites
  (`ReservationProofs.sol:715, :836, :1140-1142`).
- `feat/utxo-reservation-guards`, wallet lifecycle and storage (§5.3/§5.4):
  `beginWalletClosing` requiring a zero reservation count
  (`Wallets.sol:675-677`, repeated at `:707-709`),
  `notifyWalletMovingFundsTimeout` seizing operator stake and terminating
  (`:493-523`), and the absence of a global position counter beside
  `liveWalletsCount` (`BridgeState.sol:253`) and `reservationTotalAmount`
  (`:378`).
- `#1090`-era bytecode measurements (§5.2) are quoted from `feature-spec.md`
  §2, not re-measured here; `ReservationRouter.sol` inheriting
  `Governable, Initializable` was confirmed against source.

A comparison for decision, not a commitment of dates or scope.

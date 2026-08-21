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
| 10 | Router, **only if** EIP-170 requires it | ~736 prod / ~3,400 with tests; decided by the compiler, not by argument |

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
and ~300-500 production Go.** The router question is a bigger swing than
dissolution, and it is cheap to resolve.

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
- **Pinning fix is conditional on free slots**, where total slots are
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

## 6. How to choose

This is a volume question, not a code-size question.

| Condition | Choice | Reason |
|---|---|---|
| Design-partner scale holds (deploy-inert, tiny amount cap, cap = 1, a handful of positions) | **B** | The slot budget never depletes, the operational duty is theoretical, and B removes both a slashing vector and critical-path work |
| Any realistic chance of m1 volume | **A+** | B's capacity is a loan, and dissolution returns in m2 regardless, so it would be paid for twice |

A+ dominates rungs 0, 0+ and A outright — every step up to it is free or a
pure omission. It does **not** dominate B.

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

A comparison for decision, not a commitment of dates or scope.

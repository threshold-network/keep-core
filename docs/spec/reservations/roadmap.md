# UTXO Reservations — Milestone-Based Roadmap (minimal-first)

Status: DRAFT — scope decomposition for a **minimum-viable first release**.
Objective (2026-08-21): minimize the total code we must test, deploy, and
audit in milestone 1. Code rework/refactor is delegated to agents and is
treated as cheap — so a feature's cost is its **v1 surface mass**
(Solidity + tests + deployed size + audit exposure), not the effort to
re-add it later.

Companions: `feature-spec.md` (spec, PRs §Sources), `epic-merge-plan.md`
(stack topology, §5 audit gate), `timeline-estimate.md` (schedule),
`testing-plan.md`, `exit/alternatives.md` (§7 on custody-term cost).

## 0. The gate map that determines all scope (verified against source)

Every reservation action carries a time gate relative to
`expiresAt + gracePeriod`. Verified in `Reservation.sol` @
`feat/utxo-reservation-settlement` (#1091) and `feat/utxo-reservation-guards`
(#1094):

| Action | Time gate | Callable post-grace? |
|---|---|---|
| Redemption (direct) | `block.timestamp <= expiresAt + gracePeriod` (:618) | No — reverts |
| Redemption (vault path) | `<= expiresAt + gracePeriod` (:735) | No — reverts |
| Re-anchor (rotation/migration) | `<= expiresAt + gracePeriod` (:742) | **No — reverts** |
| `extendReservation` (renewal) | `<= expiresAt + gracePeriod` (:1083) | No — reverts |
| **Dissolution** | `block.timestamp > expiresAt + gracePeriod` (:838) | **Yes — the only one** |
| Stranding (`notifyReservationStranded`) | none (wallet `Terminated` + Active) | Yes, dead-wallet only |

Three consequences drive the entire roadmap:

1. **Dissolution is unreachable for the whole term+grace window.** It
   reverts `"Reservation term or grace period not elapsed"` until the
   deadline passes. Shipping it at launch deploys and audits code that
   cannot execute for the length of the term.
2. **Past grace, redemption *and* re-anchor both die.** Dissolution is the
   only remaining action. A stale position therefore does not merely freeze
   the owner's funds — its anchor can never move, and because wallet
   closing requires the wallet's reservation count to reach zero, it
   **pins the custodying wallet permanently** and permanently consumes
   global capacity (`reservationMaxTotalAmount`).
3. **The deadline is immovable by governance.** `termSeconds` and
   `gracePeriod` are *snapshotted at acceptance* (:174-179, applying "to
   new reservations only"). Raising either parameter later does **not**
   extend live positions. Only a contract upgrade can add an exit after the
   fact.

**Therefore a long minimum term converts dissolution from a launch
requirement into a deadline-bound m2 obligation** — and because redemption
is available throughout the term, a long term costs users nothing (the term
is a *maximum custody duration*, not a lock-up). Its real price is an
extended key-liveness obligation on the signing group per position
(`exit/alternatives.md` §7) and a longer wallet custody commitment.

## 1. Feature inventory and cut classification

| Feature | PR | Structural? (m1) | v1 surface mass | keep-core surface today | Verdict |
|---|---|---|---|---|---|
| Deposit routing → vault (`pendingReservedDeposits`) | #1088 | **Yes** | small | — | keep |
| Acceptance / creation (wallet-bound anchor proof) | #1091 + M-05 | **Yes** | core | `AnchorReservationProposal` (built) | keep |
| Two-phase settlement machine | #1091 | **Yes** | core | all four proposals need nonce rework (unwritten) | keep |
| Whole redemption (pre-grace) | #1091 | **Yes** — the launch exit path | core | `ReservedRedemptionProposal` (built) | keep |
| Re-anchor (rotation/migration, pre-grace) | #1091 | **Yes** — wallet rotation liveness | small | `ReAnchorReservationProposal` (built) + executor duty | keep |
| Backing claim≡anchor + in-kind fees | #1093 | **Yes** | medium | fee debt accounting | keep |
| Router (EIP-170 delegatecall) | #1090 | **Yes** | small | — | keep |
| Guards base (designated-wallet binding, pending/vault-migration, closing) | #1094 | **Yes** | small | — | keep |
| Stranding (`notifyReservationStranded`) | #1094 | **Yes** (anchors are live; dead-wallet valve) | small | executor duty (unwritten) | keep |
| Docs/release + frozen params | #1095 | **Yes** (governance surface) | docs | — | keep |
| **Renewal + strict expiry + rotation-window** | #1092 | No | largest removable mass (window arithmetic, `extendCustody`, exception policy, `dissolutionEligibleAt`, watchtower per-gen) | Go-light | **CUT → m2** |
| **Partial redemption (1-in-2-out)** | #1096 | No | medium-large (`isPartial` propagation, partial proof shape-match, chain-of-partials, partial timeout) | real unwritten Go | **CUT → m2** |
| **Dissolution** | #1091 + #1094 | **No — unreachable at launch** (gate :838) | medium (request/proof, `walletPendingDissolution` lock, stranding interplay) | `DissolveReservationProposal` (built) | **CUT → m2, deadline-bound** |

### The #1096 user-facing cost (unchanged, still the one argument to pull it forward)

Without partial redemption the only owner exit is whole-only redemption. A
holder wanting partial liquidity must redeem the **entire** position, then
re-deposit + re-accept a smaller one — paying acceptance fees again,
starting a new custody term, and **resetting the anchor's age** (an older
anchor is worth more under #1093: fewer re-anchor hops = less cumulative
fee loss). Accepted as an m1 product gap, flagged in release notes.

## 2. Milestone 1 — minimal viable reservations (long-term, no post-grace paths)

**Scope:** the structural set only. Cut `#1092` (renewal), `#1096`
(partial), **and dissolution**.

- **Protocol behavior:** create → redeem (whole, any time pre-grace) →
  re-anchor (rotation). Stranding covers terminated wallets. No post-grace
  action exists in the deployed surface.
- **Term:** set **long (6-9 months)** at launch, with a generous grace
  period as additional buffer. This is the mechanism that makes the
  dissolution cut safe: the first reachable post-grace moment is
  `first_acceptance + term + grace`. #1091 validates only
  `termSeconds > 0`, so this is expressible today.
- **Exits available at launch:** whole redemption (owner, throughout the
  term), re-anchor (permissionless while the source wallet is
  `MovingFunds`; governance-privileged while `Live`), stranding
  (dead-wallet valve, no time gate).
- **keep-core:** the two-phase rework must implement three retained
  proposal paths (accept, redeem, re-anchor). The dissolve proposal already
  exists in #4238 but its on-chain path is not deployed in m1 — leave it
  unwired rather than deleting it.
- **Audit:** one engagement against the reduced m1 assembly — materially
  smaller than the full feature, so the clock starts sooner.

### The hard deadline this creates (the accepted cost)

**`first_acceptance + term + grace` is a wall.** At that instant, for any
still-open position: redemption reverts, re-anchor reverts, and no
dissolution exists. The position freezes **and pins its custodying wallet**
(closing needs count zero) and permanently holds global capacity.
Governance cannot move the wall (term/grace are snapshotted per position).

Mitigation, in order:
1. **m2 must deploy dissolution before the wall.** With a 6-9 month term
   the runway is 6-9 months plus grace for code that is already designed —
   comfortable, but it is a real, dated commitment, not a nice-to-have.
2. **Residual backstop:** the Bridge is upgradeable, so a missed deadline
   is recoverable by an emergency upgrade activating dissolution — with
   governance + audit latency, and frozen positions in the interim.
3. **Monitoring:** track the earliest `expiresAt + gracePeriod` across live
   positions from launch day; it is the single date the m2 schedule is
   measured against.

**Success criteria (m1 done):**
- Reduced layout passes append-only storage-parity re-check (merge plan
  §5); m2's re-additions must be `__gap`-compatible. Dissolution's storage
  (`walletPendingDissolution`) should be laid out now even if the code path
  ships later.
- keep-core two-phase client tested (fork e2e, multi-signer sim, testnet
  drill) for the three retained paths.
- One audit against the m1 assembly; governance sign-off on frozen params,
  **including the long term + grace values and the resulting wall date**.

## 3. Milestone 2 — dissolution (deadline-bound), then renewal + partial

**Must land before the wall:**
- **Dissolution** (`#1091` request/proof path + `#1094` stranding
  interplay + the `DissolveReservationProposal` wiring in keep-core). This
  is the m2 gate; everything else in m2 can slip, this cannot.

**Should land with it (surface already designed):**
- **`#1092`** renewal / strict expiry / rotation-window /
  `dissolutionEligibleAt`. Shipping renewal alongside dissolution is the
  belt-and-braces fix: renewal lets an owner push their own deadline back
  *before* the wall, so the system no longer depends on a single dated
  delivery.
- **`#1096`** partial redemption — closes the whole-only-exit gap (§1).

Audit: second engagement against the m2 delta (or a §5 re-run over the
combined layout). Accepted as the cost of the minimal first release.

**Out of scope (no committed milestone):** stranding-compensation Tiers 0-1
beyond the `Stranded` event m1 already emits; emergency-exit (retained
reference only); FROST/Schnorr re-anchor settlement patch (§17).

## 4. Decisions confirmed (interactive walk + 2026-08-21 revision)

1. **Long-term / no-post-grace shape** — cut `#1092`, `#1096`, **and
   dissolution**; launch with a 6-9 month term so no post-grace path is
   reachable. Supersedes the earlier "Shape A" (which kept dissolution in
   m1 as mandatory); the gate map (§0) shows dissolution is unreachable at
   launch, so keeping it bought no reachable behavior.
2. **Whole-only exit gap accepted for m1** (§1) — flagged in release notes.
3. **One audit per milestone** — m1 audits the reduced assembly (fastest
   first release); m2 gets a delta re-audit.
4. **Branch stays local** — `docs/reservations-spec` not pushed, no PR.

**Carried obligations:**
- Pin the long `termSeconds` + `gracePeriod` in the frozen-spec sign-off
  ledger, and publish the derived **wall date** alongside them — they are
  user-facing deadlines with no extension mechanism in m1.
- Track dissolution as a **dated m2 commitment**, not a backlog item.
- Weigh the term length against the signing group's key-liveness
  obligation (`exit/alternatives.md` §7): longer term = more runway, but a
  longer per-position liveness commitment.

---

## Provenance

Derived 2026-08-21 from `feature-spec.md` (§3-§7, §13, §16),
`epic-merge-plan.md` (§3, §5), `timeline-estimate.md` (§2-§3), the
keep-core §13 proposal inventory, and **verified directly against source**:
`Reservation.sol` @ `feat/utxo-reservation-settlement` for the action gate
map (:618, :735, :742, :838, :1083), the acceptance-time snapshot of
`termSeconds`/`gracePeriod` (:174-179), and the sole `termSeconds > 0`
validation (:1133); `feat/utxo-reservation-guards` for
`notifyReservationStranded`'s ungated dead-wallet valve (:1363-1378). This
is a scope decomposition for decision, not a commitment of dates.
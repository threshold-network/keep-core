# UTXO Reservations — Milestone-Based Roadmap (minimal-first)

Status: DRAFT — scope decomposition for a **minimum-viable first release**.
Objective (2026-08-21): minimize the total code we must test, deploy, and
audit in milestone 1. Code rework/refactor is delegated to agents and is
treated as cheap — so the cost of a feature is its **v1 surface mass**
(Solidity + tests + deployed size + audit exposure), not the effort to
re-add it later. Everything structural is in m1; features with no
functional hole may defer.

> **Correctness constraint this roadmap is built around (verified against
> #1091, `Reservation.sol` @ `feat/utxo-reservation-settlement`):** expiry
> at the base (#1091) is **enforcing, not passive**. Redemption is gated
> `block.timestamp <= expiresAt + gracePeriod` (lines 617, 733, 1081);
> dissolution is gated `block.timestamp > expiresAt + gracePeriod`
> (line 837). They are **complementary halves of one deadline** — past
> grace, an in-kind redemption *and* a dissolution cannot both be gated
> differently; if renewal is cut, dissolution becomes the **only** post-grace
> exit. A v1 with **no renewal AND no dissolution** leaves a past-grace
> position permanently stuck — a fund trap. Consequences:
> - Cutting `#1092` makes **dissolution strictly mandatory in m1**.
> - The **term/grace values become a hard, non-extendable user deadline**
>   in a no-renewal v1 (there is no extension mechanism).

Companions: `feature-spec.md` (spec, PRs §Sources), `epic-merge-plan.md`
(stack topology, §5 audit gate), `timeline-estimate.md` (schedule,
audit-dominant), `testing-plan.md`.

## 0. Feature inventory and cut classification

| Feature | PR | Structural? (must be in m1) | v1 surface mass (Solidity+tests+deploy) | keep-core surface today | Cut verdict |
|---|---|---|---|---|---|
| Deposit routing → vault (`pendingReservedDeposits`) | #1088 | **Yes** | small | — | keep |
| Acceptance / creation (wallet-bound anchor proof) | #1091 + M-05 | **Yes** | core | `AnchorReservationProposal` (built) | keep |
| Two-phase settlement machine (authorize-then-prove) | #1091 | **Yes** | core | all four proposals carry nonce (unwritten rework) | keep |
| Whole redemption (pre-grace only) | #1091 | **Yes** | core | `ReservedRedemptionProposal` (built) | keep |
| Re-anchor (rotation/migration) | #1091 | **Yes** | small | `ReAnchorReservationProposal` (built) + executor duty | keep |
| Backing claim≡anchor + in-kind fees | #1093 | **Yes** | medium | fee debt accounting | keep |
| Router (EIP-170 delegatecall) | #1090 | **Yes** | small | — | keep |
| Guards base (designated-wallet binding, pending/vault-migration, closing) | #1094 | **Yes** | small | — | keep |
| Stranding (`notifyReservationStranded`, H-06) | #1094 | **Yes** (anchors are live) | small | executor duty (unwritten) | keep |
| Docs/release + frozen params | #1095 | **Yes** (governance surface) | docs | — | keep |
| **Renewal + strict expiry + rotation-window** | #1092 | No | largest removable mass (window arithmetic, `extendReservation`/`extendCustody`, exception-policy layer, `dissolutionEligibleAt` snapshots, watchtower per-gen) | Go-light (executor bookkeeping/prioritization, no new signed UTXO) | **CUT → m2** — *forces dissolution into m1* |
| **Partial redemption (1-in-2-out)** | #1096 | No | medium-large (`isPartial` propagation, partial proof shape-match, chain-of-partials, partial timeout) | real unwritten Go (partial assembler + `requestPartialReservedRedemption`) | **CUT → m2**, *see user-facing cost* |
| Dissolution | #1091 + #1094 | **STRICTLY REQUIRED if #1092 is cut** (only post-grace exit) | medium (request/proof, `walletPendingDissolution` lock, stranding interplay) | `DissolveReservationProposal` (built) | **KEEP in m1** (mandatory, not optional) |

### Structural set rationale (why these are non-negotiable)

- The two-phase machine, backing invariant, router, guards, and stranding
  are not optional extras: acceptance writes anchors to a live store, and
  the settlement/backing/guard/stranding machinery is what makes a live
  anchor safe. Deferring any of them is a v1 without a working reservation
  system, not a smaller one.
- Docs/frozen params are the governance sign-off surface; there is no v1
  without them.
- **Dissolution is in the structural set because of the expiry gate, not
  because it is desirable scope.** In a no-renewal v1, redemption simply
  reverts past `expiresAt + gracePeriod` (§0 top); dissolution is the only
  way a position exits after that hard deadline. Ship it.

### The #1096 user-facing cost (first question product asks)

**Without partial redemption, the only exit from a reservation is
whole-only redemption** (and re-anchor/rotation, and dissolution past
grace). A holder who wants *partial* liquidity must:
1. redeem the **entire** position (paying the full redemption fee and
   losing the in-kind lineage), then
2. re-deposit + re-accept a smaller reservation — paying **acceptance
   fees again** and starting a **new custody term**.

That also **resets the anchor's age** — an older anchor is worth more
(fewer re-anchor hops = less cumulative fee loss under the #1093 backing
model). So cutting partial taxes liquidity-chunking users twice (fees +
term + anchor age). This is the **only real argument for pulling #1096
forward into m1**: it exists to serve that user, and m1 carries it as an
accepted product gap until m2. Whether that tradeoff is acceptable is a
product call, not a code-cost call.

### The dissolution decision is NOT open — it is dictated by the cut

Because redemption is hard-gated at `expiresAt + gracePeriod` and #1092's
renewal is the only extension mechanism, **cutting #1092 forces dissolution
into m1** as the sole post-grace exit. The roadmap therefore does **not**
offer "dissolution out of v1" as a coherent option — that variant is a fund
trap. The only two coherent minimal-v1 shapes are:

- **A (recommended):** cut `#1092` + `#1096`; keep dissolution. v1 =
  create → redeem (pre-grace) → re-anchor → dissolve (post-grace). Fixed
  non-extendable term/grace; dissolution is the post-grace unwind.
- **B:** keep `#1092` (renewal) in v1 so an owner can always extend before
  grace, then dissolution becomes optional/later. Larger m1 surface, but
  term is no longer a hard deadline and dissolution can defer to m2.

Pick A for smallest surface; pick B only if a non-extendable hard term is
unacceptable and the extra #1092 surface is worth keeping. Either way,
**A-and-cut-dissolution is forbidden** (stuck positions).

## 1. Milestone 1 — minimal viable reservations

**Scope (recommended, Shape A):** all structural features + **dissolution**;
cut `#1092` (renewal/expiry/rotation-window) and `#1096` (partial
redemption).

- Protocol behavior: create → redeem (whole, **pre-grace only**) →
  re-anchor → dissolve (**post-grace only**).
- Expiry: enforcing legacy `expiresAt + gracePeriod` (the #1091 model) —
  **hard, non-extendable** deadline with no renewal. Term/grace are a
  governance-set user-facing deadline; document them as such.
- Exit paths: whole redemption closes a position before grace; dissolution
  releases a pinning anchor after grace; stranding guards terminated
  wallets. **No position is ever stuck**: every state has exactly one
  exit.
- keep-core: the two-phase rework (`#4238` follow-up) must implement the
  four nonce-carrying proposal paths currently stubbed (accept, redeem,
  re-anchor, dissolve) — this is the critical path to launch (spec §16
  item 3). Partial/renewal proposal surfaces are **not** part of this
  first client.
- Audit (§5 of the merge plan): runs against the **reduced** m1 assembly —
  smaller than the full feature, so it is cheaper and its clock starts
  sooner (the latency win the objective is buying).

**Success criteria (m1 done):**
- Reduced v1 layout passes the append-only storage-parity re-check
  (merge plan §5 note — the m2 re-additions must be `__gap`-compatible;
  dissolution's `walletPendingDissolution` + `expiresAt`/`gracePeriod`
  must be present for the post-grace path).
- keep-core two-phase client tested (fork e2e, multi-signer sim, testnet
  drill — timeline §2) for the four retained paths.
- One audit against the m1 assembly; governance sign-off on frozen params,
  **including the hard term/grace values** (these are now user-facing
  deadlines, so the frozen-spec sign-off ledger must pin them).

## 2. Milestone 2 — renewal + partial (re-add on a `__gap`-compatible delta)

**Scope:** `#1092` (renewal, strict expiry, rotation-window,
`dissolutionEligibleAt`) + `#1096` (partial redemption), re-introduced on
the m1 `__gap` delta — agents re-add the previously-cut code.

- Protocol behavior adds: permissionless bounded renewal (window < term,
  non-stacking), per-generation watchtower veto integration, and the
  1-in-2-out partial path with chain-of-partials. Renewal restores the
  extension mechanism #1092's model supplies; dissolution remains as the
  post-expiry unwind (now via `dissolutionEligibleAt`).
- Product gap closes: partial-liquidity users no longer pay the recover-
  and-reaccept cost (§0 #1096 row); the term stops being a hard deadline.
- keep-core: add the partial assembler + `isPartial`-aware proof and the
  renewal-prioritization executor bookkeeping; `#4238`'s four core
  proposal types are unchanged (built in m1).
- Audit: second engagement against the m2 delta (or re-run of the §5
  checklist over the combined layout). Accepted as the cost of the minimal
  first release — reworked/delegated code makes this cheap relative to the
  m1 latency saved.

**Out of scope (no committed milestone):** stranding-compensation Tiers
0-1 beyond what m1's `Stranded` event already emits; emergency-exit (locked
as retained reference); FROST/Schnorr re-anchor settlement patch (tracks
§17, lands when FROST activates).

## 3. Open decisions to confirm before this roadmap is authoritative

1. **Shape A (cut #1092 + #1096, keep dissolution) vs Shape B (keep #1092,
   defer dissolution).** This roadmap recommends A; B is only for a team
   that rejects a non-extendable hard term. **Shape A-without-dissolution
   is not offered** — it produces permanently stuck past-grace positions
   (verified: redemption reverts past grace at #1091:617/733/1081, and
   nothing else releases a pinning anchor).
2. **Is the whole-only exit gap (cutting #1096) acceptable for the first
   release?** (recommended: yes, given the latency/surface objective; flag
   it in release notes so partial-liquidity users know).
3. Under Shape A, **pin the hard term + grace values** in the frozen-spec
   sign-off ledger — they are user-facing deadlines with no extension, not
   internal tuning knobs.
4. Confirm the **one-audit-per-milestone** model (vs. deferring all
   features behind one assembled audit) — this roadmap assumes the former.

---

## Provenance

Derived 2026-08-21 from `feature-spec.md` (§3-§7, §13, §16),
`epic-merge-plan.md` (§3, §5), `timeline-estimate.md` (§2-§3), the
keep-core §13 proposal inventory, and **verified directly against #1091
(`Reservation.sol` @ `feat/utxo-reservation-settlement`)** for the
expiry/dissolution gating (lines 617, 733, 837, 1081) that makes
dissolution mandatory in a no-renewal v1. Feature-to-PR mapping matches
spec §Sources and the merge-plan stack table. This is a scope
decomposition for decision, not a commitment of dates.
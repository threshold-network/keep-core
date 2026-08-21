# UTXO Reservations — Milestone-Based Roadmap (create-only first release)

Status: DRAFT — for review.
Objective (2026-08-21): ship the smallest surface that can be tested,
deployed, and audited, and make every later fix reachable by upgrade.
Agent-delegated rework is treated as cheap, so a feature's cost is its
**v1 surface mass** (Solidity + tests + deployed size + audit exposure),
not the effort to re-add it later.

**Milestone 1 is a rails release, not a product release.** Users can create
reservations. They cannot redeem in-kind, re-anchor, renew, or dissolve.

Companions: `feature-spec.md` (spec, §Sources), `epic-merge-plan.md`
(stack topology, §5 audit gate, §11 deploy-inert pattern),
`timeline-estimate.md`, `testing-plan.md`, `exit/alternatives.md` (§7,
custody-term cost).

## 0. Source-verified facts that determine the whole scope

All line references are `Reservation.sol` @ `feat/utxo-reservation-settlement`
(#1091) unless noted.

### 0.1 The action gate map

| Action | Time gate | Callable post-grace? |
|---|---|---|
| Redemption (direct) | `block.timestamp <= expiresAt + gracePeriod` (:618) | No |
| Redemption (vault path) | `<= expiresAt + gracePeriod` (:735) | No |
| Re-anchor (rotation/migration) | `<= expiresAt + gracePeriod` (:742) | No |
| `extendReservation` (renewal) | `<= expiresAt + gracePeriod` (:1083) | No |
| Dissolution | `block.timestamp > expiresAt + gracePeriod` (:838) | Yes — the only one |
| Stranding (`notifyReservationStranded`) | none (wallet `Terminated` + Active) | Yes, dead-wallet only |

### 0.2 "Create" is itself two-phase

`requestReservationAcceptance` (:401) plus an acceptance proof. So m1
necessarily ships the action record, `ActionType.Acceptance`, the
designated-wallet binding, `notifyReservationActionTimeout` (:911, which
releases the capacity and locks reserved at request time), and
stale-deposit cleanup. There is no cheaper single-call create.

### 0.3 Minted tBTC is an ordinary fungible claim

The contract states this itself: on `Stranded`, *"the owner's minted balance
remains an ordinary pooled claim; the anchor is no longer tracked"*
(:89-91); after dissolution, *"the owner's minted balance simply remains an
ordinary pooled claim"* (:807-809).

So a create-only user is **not trapped** — they can sell their tBTC or
redeem it through the ordinary pooled path like any holder. What m1
withholds is the *in-kind* guarantee (getting their specific coin back),
which is the product's entire value-add. Hence: rails, not product.

The global invariant survives that exit. If an owner pooled-redeems X:
supply `S−X`, pooled `P−X`, anchors unchanged at `A`; since `S = P + A`,
`S−X = (P−X) + A`. ✓ The anchor's BTC simply backs other pooled claims
until it is dissolved into the pool later. **The exposure is pooled
liquidity, not solvency**, and it is bounded by the total reserved cap.

### 0.4 Bookkeeping-only closes are unsound (a rejected design)

`closeReservation` (:1183-1192) only decrements the wallet count, subtracts
`anchorAmount` from `reservationTotalAmount`, and marks `Closed`. That is
**loss recognition**, valid for `Stranded` solely because a Terminated
wallet's BTC is already presumed gone.

Applying it to a Live/MovingFunds wallet would *create* the loss: the anchor
UTXO physically remains outside `mainUtxo`, no Bridge path authorizes its
spend once the reservation is `Closed`, and the owner keeps their claim —
a genuine shortfall of `anchorAmount`, not the liquidity mismatch of §0.3.

This is why dissolution carries a proof cycle at all:
`action.actionDataHash = wallet.mainUtxoHash` and
`action.sourceAnchorUtxoHash = anchorUtxoHash(reservation)` (:891-892) —
merging an anchor into pooled backing is a real Bitcoin transaction with
SPV proof. **Any sound unpin must move BTC, not just adjust books.** A
proposed "ten-line governance force-close" was evaluated and rejected on
this basis.

### 0.5 Term and grace are snapshotted, so deadlines are immovable

`termSeconds` and `gracePeriod` are snapshotted at acceptance and apply "to
new reservations only" (:174-179). Raising either parameter later does not
extend live positions. `updateReservationParameters` validates only
`termSeconds > 0` (:1133). Only a contract upgrade can add an exit to an
existing position.

## 1. Milestone 1 — create-only rails

### 1.1 Ships

| Piece | PR | Why it cannot wait |
|---|---|---|
| Deposit routing + permanent reveal-time classification | #1088 | The entry gate; classification is one-way by design |
| Acceptance request + proof | #1091 | This *is* "create" (§0.2) |
| Designated-wallet binding (M-05) | #1094 | Without it a Byzantine wallet anchors deposits to itself, and an existing anchor cannot be undone |
| Mint + backing accounting (`mintedAmount`/`anchorAmount`) | #1093 | Supply exists from day one; minted tBTC cannot be re-accounted retroactively |
| Acceptance timeout + stale-deposit cleanup | #1091/#1088 | Otherwise every failed acceptance permanently leaks capacity |
| Router (EIP-170 delegatecall) | #1090 | Structural at the bytecode limit |
| Caps (`reservationMaxTotalAmount`, `maxReservationsPerWallet`, `reservationMinAmount`) | #1093 | The blast-radius instrument, and the only knob tunable **without** an upgrade |
| Stranding (`notifyReservationStranded`) | #1094 | Small; prevents a permanent capacity leak when a wallet terminates |
| **Complete storage layout** | all | See §2.1 — the crux of upgradeability |

### 1.2 Does not ship

Whole redemption, partial redemption (#1096), re-anchor, renewal (#1092),
dissolution, watchtower veto integration for reserved paths, compensation
module. All are pure logic over records m1 already writes (§2.1).

### 1.3 Launch posture (decided)

**Deploy inert, then activate for design partners under a tiny cap.**
Deploy and audit with the reservation vault inactive (`epic-merge-plan.md`
§11's deploy-inert-then-activate pattern), then flip the governance switch
for a controlled cohort: small `reservationMaxTotalAmount`, and
`maxReservationsPerWallet = 1` so pinning is confined to wallets you can
coordinate with. No position exists until deliberately enabled.

### 1.4 Term (decided): 12 months + generous grace

In create-only the term is a **promise clock**, not a dissolution deadline.
Redemption is gated `<= expiresAt + gracePeriod` (:618/:735), so if in-kind
redemption has not shipped before that line, the first cohort's in-kind
option **expires silently** and their only exit was always the pool.

12 months is roughly double a realistic redemption ship date (m1 audit +
governance, then m2 build + audit), giving real margin. At design-partner
caps the extra key-liveness obligation on signing groups
(`exit/alternatives.md` §7) is negligible, so the usual argument against
long terms barely applies. The clock is unextendable in m1 (§0.5), so this
is a user-facing commitment: publish the derived date with the frozen
parameters.

### 1.5 Accepted risk: wallet pinning, bounded only by caps (decided)

No unpin mechanism ships. An anchor cannot leave its wallet, and wallet
closing requires the reservation count to reach zero, so **a wallet that
custodies a reservation cannot complete `MovingFunds` until m2**. The cost
lands on that wallet's operators (locked stake), not on the protocol's
books — this is accounting-sound, an operational hostage rather than a
shortfall.

Considered and rejected for m1: governance-triggered dissolution with the
time gate dropped (sound, but it pulls in the request path, the dissolution
branch of the proof dispatcher, and its timeout path); re-anchor (only
relocates the pin); dissolution as designed (unreachable during the term
per §0.1).

Bounding measures, in order:
1. `maxReservationsPerWallet = 1` and a small total cap — both governance
   params, tunable without an upgrade.
2. Design-partner-only activation, so the pinnable wallet set is known.
3. Monitor which wallets hold anchors; treat "a pinned wallet needs to
   retire" as the trigger to prioritize m2.

## 2. The upgradeability contract

This is what makes "fix edge cases later" true rather than hopeful.

### 2.1 Ship storage-complete, behavior-minimal

Keep the full `ReservationRequest` struct and **populate every field at
acceptance** — `termSeconds`, `gracePeriod`, `expiresAt`, `anchorAmount`,
`requestNonce`, state — even though m1 reads almost none of them.

Failure mode if skipped: m2 adds `expiresAt`, legacy records read `0`,
and every m1 position is instantly past-grace — redemption (gated
`<= expiresAt + gracePeriod`) would revert **forever for exactly the
earliest users**. Storage-complete now means no data migration ever, and
every deferred feature becomes pure logic addition.

Also lay out dissolution's `walletPendingDissolution` slot now, even though
the path ships later, so m2 is a clean `__gap`-compatible add.

### 2.2 The §11 no-migration rule is satisfied by construction

The hard rule is that no live `ReservationAction` may exist on an
intermediate storage layout. Acceptance actions are **transient** (bounded
by `reservationActionTimeout`, hours); positions are long-lived. Deferring
every long-lived settlement path means the upgrade is an operational
sequence, not an architectural problem:

> stop new acceptances → drain in-flight actions (≤ timeout) → upgrade →
> resume.

This is create-only's strongest structural argument.

### 2.3 The Bitcoin side is the only true one-way door

Anchor shape (1-in-1-out to the designated wallet), the reveal-script
commitment, and anchor identification (`anchorTxHash` / output index) are
on Bitcoin for every accepted position and **cannot be re-shaped by an
upgrade**. Pre-launch scrutiny belongs here, not on the settlement logic
that upgrades can freely replace.

### 2.4 What upgrades can and cannot fix

| Fixable later by upgrade | Not fixable — must be right at launch |
|---|---|
| All settlement paths (redeem, partial, re-anchor, renew, dissolve) | Bitcoin anchor format + reveal-script commitment (§2.3) |
| Watchtower veto integration for reserved paths | Reveal-time permanent classification (one-way by design) |
| Fee-model refinements (fields already present) | Backing accounting correctness for already-minted tBTC |
| Compensation / stranding-liability module | Designated-wallet binding (existing anchors cannot be undone) |
| Cap values (governance params, no upgrade needed) | Storage layout compatibility (§2.1) |
| Removing the pinning risk (§1.5) | Term/grace on existing positions (§0.5) |

## 3. Milestone 2 — the in-kind exit (the promised feature)

**Gate: must land before the earliest `expiresAt + gracePeriod`** (§1.4),
otherwise the first cohort's in-kind option lapses.

1. **Whole redemption** — the value-add m1 withholds; makes reservations a
   real product.
2. **An unpin mechanism** — re-anchor (proper rotation) and/or dissolution;
   clears the §1.5 hostage.
3. **Renewal (#1092)** — lets owners move their own deadline, so the
   promise clock stops depending on a single dated delivery.

Then: partial redemption (#1096), watchtower veto integration, compensation.

Audit: second engagement against the m2 delta (or a §5 re-run over the
combined layout) — the accepted cost of a minimal first release.

## 4. Decisions confirmed (interactive walks, 2026-08-21)

1. **Create-only m1** — users can create; no redeem, renew, re-anchor, or
   dissolve. Supersedes the earlier "long-term no-post-grace" shape, which
   still shipped whole redemption.
2. **No unpin mechanism in m1** — caps are the sole bound; wallet pinning
   accepted as an operational risk (§1.5).
3. **Deploy inert, then activate for design partners under a tiny cap**
   (§1.3).
4. **Term 12 months + generous grace** (§1.4).
5. **One audit per milestone** — m1 audits the reduced assembly; m2 a delta.
6. **Branch stays local** — `docs/reservations-spec` not pushed, no PR.

**Carried obligations:**
- Publish the term/grace values **and the derived promise-clock date** in
  the frozen-spec sign-off ledger.
- Enforce storage-completeness at acceptance (§2.1) — the single highest-
  leverage implementation rule in this plan.
- Monitor anchored wallets; a pinned wallet needing retirement escalates
  m2 (§1.5).
- Treat the Bitcoin anchor format as launch-final (§2.3).

## 5. Open questions for review

1. Is a rails release with **no enforceable in-kind exit** acceptable to put
   in front of design partners, given the promise rests on an m2 upgrade?
2. Concrete cap values for activation (total reserved amount; per-wallet
   count is decided at 1).
3. Should reservation-eligible **wallets be allowlisted** at activation?
   The depositor picks the designated wallet at reveal, so today any Live
   wallet can be pinned; an allowlist would confine §1.5's risk further but
   adds surface not currently in any PR.

---

## Provenance

Derived 2026-08-21 from `feature-spec.md` (§3-§7, §13, §16),
`epic-merge-plan.md` (§3, §5, §11), `timeline-estimate.md` (§2-§3), and the
keep-core §13 proposal inventory. **Verified directly against source**:
`Reservation.sol` @ `feat/utxo-reservation-settlement` for the action gate
map (:618, :735, :742, :838, :1083), two-phase acceptance (:401) and action
timeout (:911), the fungible-pooled-claim semantics (:89-91, :807-809), the
`closeReservation` bookkeeping helper (:1183-1192), dissolution's proof
payload (:891-892), acceptance-time snapshotting of `termSeconds`/
`gracePeriod` (:174-179), and the sole `termSeconds > 0` validation (:1133);
`feat/utxo-reservation-guards` for `notifyReservationStranded` (:1363-1378).
This is a scope decomposition for decision, not a commitment of dates.
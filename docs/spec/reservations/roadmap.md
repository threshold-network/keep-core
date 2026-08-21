# UTXO Reservations — Milestone-Based Roadmap (create-only first release)

Status: DRAFT — for review. Revised 2026-08-21 after a source re-verification
that **reversed the earlier "cut #1092" decision** (§0.1).

Objective: ship the smallest *reachable* surface, and make every later fix
an upgrade rather than a migration. Agent-delegated rework is cheap, so a
feature's cost is its v1 surface mass — but a PR's cost is not the same as a
feature's cost, and §0.1 is where that distinction bites.

**Milestone 1 is a rails release, not a product release.** Users can create
reservations. They cannot redeem in-kind or renew.

Companions: `feature-spec.md`, `epic-merge-plan.md` (stack topology, §5
audit gate, §11 deploy-inert pattern), `timeline-estimate.md`,
`testing-plan.md`, `exit/alternatives.md` (§7, custody-term cost).

## 0. Source-verified facts that determine the scope

Citations are `Reservation.sol` unless noted, with the branch named — this
matters, because **the expiry model differs across the stack**.

### 0.1 #1092 is structural to the upper stack, not an additive layer

On `feat/utxo-reservation-settlement` (#1091) actions gate on
`expiresAt + gracePeriod`: redemption `<=` (:618, :735), re-anchor `<=`
(:742), `extendReservation` `<=` (:1083), dissolution `>` (:838).

On `feat/utxo-reservation-backing` (#1093) that model **is gone**:

- `dissolutionEligibleAt` is a struct field (:186), set to
  `expiresAt + reservationDissolutionDelay` whenever a term is granted, with
  explicit snapshot semantics: *"later governance changes never move the
  eligibility time of a term already granted"* (:180-184).
- The gates were rewritten: pre-eligibility actions require
  `block.timestamp < reservation.dissolutionEligibleAt` (:766); dissolution
  requires `>= dissolutionEligibleAt` (:880).
- `gracePeriod` **no longer exists**; `reservationDissolutionDelay` and
  `reservationRenewalWindowSeconds` are governance parameters (:308-314,
  :1212-1218).
- `updateReservationParameters` **requires**
  `reservationRenewalWindowSeconds > 0 && < reservationTermSeconds`
  (:1232-1236).

**Consequences.** #1093/#1094/#1095 are built on post-#1092 semantics, and
the parameter validator refuses to configure a system with no renewal
window. Excising #1092 would mean reverting gate rewrites inside three
downstream PRs and removing a struct field — intra-PR surgery on reviewed
code, not a rebase. Since m1 needs #1093 (backing, caps) and #1094 (guards,
stranding), **#1092's code must ship.** Create-only is therefore achieved by
controlling *reachability* (§0.2), not by omitting PRs.

### 0.2 Caller gates are the create-only control surface

Verified on both #1091 and #1093 branches:

| Path | Gate | Reachable in m1? |
|---|---|---|
| Acceptance (`requestReservationAcceptance`) | permissionless (:401) | **Yes** — this is the product |
| Redemption (`requestReservedRedemption`) | `msg.sender == self.reservationVault` (#1091 :584, #1093 :614) | **No** — m1 vault exposes no entry point |
| Renewal (`extendReservation`) | `msg.sender == self.reservationVault` (#1091 :1064, #1093 :1133) | **No** — same |
| Re-anchor (`requestReservationReanchor`) | permissionless while source is `MovingFunds`; `privileged` required while `Live` (:718-757) | **Yes** — and this is desirable (§1.5) |
| Dissolution (`requestReservationDissolution`) | permissionless, post-eligibility only (:824, :880) | **No** for ~12 months — self-gating by term |
| Action timeout | permissionless (:911) | **Yes** — required cleanup |
| Redemption veto | `msg.sender == self.redemptionWatchtower` (:1017) | Vacuous — no redemptions exist |
| Stranding | wallet `Terminated` + Active, no time gate (#1094 :1363-1378) | **Yes** — capacity valve |

Because redemption and renewal are **vault-gated**, a minimal m1 vault that
exposes neither makes both unreachable **without touching Bridge code**.
This is the cheapest possible scope control, and it makes m2 a *vault-side*
change (§3).

### 0.3 Minted tBTC is an ordinary fungible claim

The contract says so: on `Stranded`, *"the owner's minted balance remains an
ordinary pooled claim; the anchor is no longer tracked"* (:89-91); after
dissolution, *"the owner's minted balance simply remains an ordinary pooled
claim"* (:807-809).

So a create-only user is **not trapped** — they can sell their tBTC or
redeem via the ordinary pooled path. m1 withholds the *in-kind* guarantee,
which is the product's value-add. Hence: rails, not product.

The global invariant survives that exit: if an owner pooled-redeems X, then
supply `S−X`, pooled `P−X`, anchors unchanged at `A`; since `S = P + A`,
`S−X = (P−X) + A`. ✓ The exposure is **pooled liquidity, not solvency**,
bounded by the total reserved cap.

### 0.4 Bookkeeping-only closes are unsound (rejected design)

`closeReservation` (#1091 :1183-1192) only decrements the wallet count,
subtracts `anchorAmount` from `reservationTotalAmount`, and marks `Closed`.
That is loss *recognition* — valid for `Stranded` solely because a
Terminated wallet's BTC is already presumed gone.

On a Live wallet it would *create* the loss: the anchor UTXO remains outside
`mainUtxo`, no Bridge path authorizes its spend once `Closed`, and the owner
keeps their claim — a real `anchorAmount` shortfall, not §0.3's liquidity
mismatch. This is why dissolution carries a proof cycle at all:
`action.actionDataHash = wallet.mainUtxoHash` and
`action.sourceAnchorUtxoHash = anchorUtxoHash(reservation)` (:891-892).
**Any sound unpin must move BTC with an SPV proof.** A proposed "ten-line
governance force-close" was evaluated and rejected on this basis.

### 0.5 "Create" is itself two-phase, and already storage-complete

`requestReservationAcceptance` (:401) plus an acceptance proof — so m1
necessarily ships the action record, `ActionType.Acceptance`, the
designated-wallet binding, and `notifyReservationActionTimeout` (:911).

Acceptance **already populates every field**: `ReservationProofs.sol:448-463`
writes `owner`, `mintedAmount`, `acceptedAt`, `walletPubKeyHash`,
`anchorAmount`, `expiresAt`, `anchorTxHash`, `anchorTxOutputIndex`, `state`,
`termSeconds`, and `gracePeriod` (on #1093+, `dissolutionEligibleAt` per
§0.1). So storage-completeness is **already satisfied** — it is an invariant
to *preserve* under any re-scope, not a gap to close (§2.1).

## 1. Milestone 1 — create-only rails

### 1.1 Ships (whole PRs, no intra-PR surgery)

`#1088` (+`#1102` fold) routing and permanent reveal-time classification ·
`#1090` router · `#1091` settlement machine · `#1092` renewal/expiry model
(**required** by §0.1) · `#1093` backing, fees, caps · `#1094` guards,
designated-wallet binding, stranding · `#1095` docs and frozen params.

### 1.2 Deferred

- **`#1096` partial redemption** — the only clean PR omission in the stack.
- **Redemption and renewal *reachability*** — code ships (§0.1) but the m1
  vault exposes no entry point (§0.2), so neither is callable.
- **Dissolution reachability** — self-gates for ~12 months via the term.

### 1.3 Launch posture (decided)

Deploy inert, then activate for design partners under a tiny cap
(`epic-merge-plan.md` §11's deploy-inert-then-activate). Small
`reservationMaxTotalAmount`; `maxReservationsPerWallet = 1`. No position
exists until governance flips the switch.

### 1.4 Parameters (decided, restated in upper-stack vocabulary)

`gracePeriod` does not exist on the shipped stack (§0.1), so the earlier
"12 months + generous grace" becomes:

| Parameter | m1 value | Note |
|---|---|---|
| `reservationTermSeconds` | **12 months** | The promise clock (below) |
| `reservationDissolutionDelay` | generous | Sets `dissolutionEligibleAt = expiresAt + delay`; snapshotted per term granted (:180-184) |
| `reservationRenewalWindowSeconds` | `> 0 && < term` | **Cannot be zero** (:1232-1236); unreachable anyway since renewal is vault-gated |
| `reservationMaxTotalAmount` | tiny | Bounds pooled-liquidity exposure (§0.3) |
| `maxReservationsPerWallet` | 1 | Bounds pinning blast radius |
| `reservationMinAmount` | partner-appropriate | — |

**The term is a promise clock.** Redemption gates on pre-eligibility
(§0.1/§0.2), so if in-kind redemption has not shipped before
`dissolutionEligibleAt`, the first cohort's in-kind option lapses silently
and their only exit was always the pool. 12 months is roughly double a
realistic m2 date. The clock is unextendable in m1 (renewal unreachable), so
publish the derived date with the frozen parameters.

### 1.5 Wallet pinning is solved, not accepted (revised)

The earlier decision accepted pinning because no unpin mechanism would
ship. That premise is now false: **`#1091` ships, so re-anchor ships, and it
is permissionless while the source wallet is `MovingFunds`** (:718-757) —
exactly the retiring-wallet case. A wallet needing to retire can have its
anchors re-anchored to a Live wallet by anyone, dropping its reservation
count so closing can complete.

Residual conditions, not risks:
- Re-anchor needs the keep-core executor to sign and prove the hop (§4), so
  the *client* must implement re-anchor even though it is not user-facing.
- While `Live`, re-anchor requires `privileged` — governance-driven rotation
  only. Acceptable at design-partner scale.

## 2. The upgradeability contract

### 2.1 Preserve storage-completeness (already true)

Acceptance writes every field today (§0.5). The rule is therefore
**defensive**: any re-scope of the acceptance path must keep writing
`termSeconds`, `expiresAt`, and `dissolutionEligibleAt`. If a future edit
dropped them, m2 would read zeros and every m1 position would be instantly
dissolution-eligible — permanently barring the earliest users from in-kind
redemption. Add a test asserting the fields are non-zero after acceptance.

### 2.2 m2 needs no Bridge upgrade for redemption

Because redemption and renewal are vault-gated (§0.2) and their Bridge-side
code ships in m1, enabling them is a **vault-side change**, not a Bridge
storage migration. Consequences:

- `epic-merge-plan.md` §11's no-live-action-on-intermediate-layout rule is
  satisfied by construction for the redemption rollout — there is no Bridge
  layout change at all.
- The only m1 in-flight records are acceptance and re-anchor actions, both
  transient (bounded by `reservationActionTimeout`).
- Confirm whether the vault is proxy-upgradeable. If it is not, m2 requires
  re-pointing `reservationVault`, which `feature-spec.md` §15 already flags
  as hazardous while any reserved deposit could still settle late. **This is
  the single most important unknown in the plan** (§5).

### 2.3 The Bitcoin side is the only true one-way door

Anchor shape (1-in-1-out to the designated wallet), the reveal-script
commitment, and anchor identification (`anchorTxHash` / output index) are on
Bitcoin for every accepted position and cannot be re-shaped by an upgrade.
Pre-launch scrutiny belongs here.

## 3. Optimal merge order

**Keep the stack intact; cut only `#1096`.** Per §0.1 there is no cheaper
cut, and per §0.2 no cut is needed to reach create-only behavior.

| Step | PR | Action | Gate before advancing |
|---|---|---|---|
| 1 | `#1088` | Merge (already carries `#1102`'s 30-finding fold) | Storage layout final; classification permanence tested |
| 2 | `#1090` | **Rebase over the `#1102` fold** — currently CONFLICTING | Router parity, selector disjointness, no standalone authority |
| 3 | `#1091` | Rebase; **reconcile `reservationsByAnchorUtxo`** (§4) | Two-phase machine; acceptance writes all fields (§2.1) |
| 4 | `#1092` | Rebase — required by §0.1, reachability closed by the vault | `dissolutionEligibleAt` snapshot semantics; window `< term` |
| 5 | `#1093` | Rebase | Backing invariant claim ≡ anchor; caps enforced |
| 6 | `#1094` | Rebase; **same anchor-index reconciliation** (§4) | Designated-wallet binding; stranding releases capacity |
| 7 | `#1095` | Rebase; update frozen params to §1.4 and document create-only | Frozen-param sign-off incl. promise-clock date |
| — | `#1096` | **Defer to m2**; retarget after step 7 lands | — |

Then: one audit against the assembled m1 (`epic-merge-plan.md` §5), deploy
inert, activate per §1.3.

**m2 order:** vault upgrade exposing redemption (no Bridge change, §2.2) →
`#1096` partial (Bridge change, its own audit delta).

## 4. Suggested edits to existing PRs

1. **`#1090` — rebase (blocking).** Only CONFLICTING PR; it is the head of
   the chain, so nothing above it can merge first. Mechanical.
2. **`#1091` — reconcile the anchor index (substantive).**
   `ReservationProofs.sol:465` writes `reservationsByAnchorUtxo`, which
   `#1102` removed from the merged base in favour of `spentMainUTXOs`.
   Decide one mechanism and apply it consistently.
3. **`#1094` — same reconciliation.** It declares and uses the anchor
   mapping for `strandReservation`. This and item 2 are the same defect
   surfacing twice; fix them together or the stranding path breaks.
4. **`#1092` — no code change, but retract the "cut" note** wherever the
   docs claim it is omitted (§0.1). Its parameters need m1 values (§1.4).
5. **`#1093` — no structural change.** Confirm cap enforcement is reachable
   with renewal unreachable (caps are checked at acceptance, so they should
   be — verify, do not assume).
6. **`#1095` — content edits.** Frozen params per §1.4, the create-only
   surface, the reachability matrix (§0.2), and the promise-clock date.
7. **`#1096` — retarget only**, after step 7.
8. **keep-core `#4238` — re-scope.** It models the single-phase whole-
   redemption design and predates the two-phase machine. For m1 it needs
   acceptance and re-anchor as nonce-carrying proposals; redemption and
   dissolution proposals can stay unwired.

## 5. Implementation gaps for m1 (high level)

1. **keep-core two-phase client** — acceptance and re-anchor only, with
   nonce-carrying proposals, executor duties, and regenerated ABI bindings.
   This is the critical path (`feature-spec.md` §16 item 3).
2. **Minimal m1 `ReservationVault`** — exposes the acceptance/credit path
   and *no* redemption or renewal entry point (§0.2). **Confirm proxy
   upgradeability** (§2.2) — it determines whether m2 is an upgrade or a
   hazardous re-point.
3. **Anchor-index reconciliation** across `#1091`/`#1094` (§4 items 2-3).
4. **Parameter and activation wiring** — §1.4 values, the deploy-inert
   switch, and governance runbook steps.
5. **Executor duty: re-anchor on rotation** — prompted by
   `WalletMovingFunds`, since §1.5's unpinning depends on it being performed.
6. **Monitoring** — anchored wallets (pinning watch), earliest
   `dissolutionEligibleAt` (promise clock), pooled-liquidity exposure vs cap.
7. **Tests** — acceptance happy path; timeout and stale-deposit cleanup; cap
   enforcement; a storage-completeness assertion (§2.1); and a
   **reachability test** proving redemption and renewal revert for every
   caller other than the vault.

## 6. Decisions confirmed

1. **Create-only m1** — users create only; redemption and renewal exist
   on-chain but are unreachable (§0.2).
2. **Stack ships intact, only `#1096` deferred** — supersedes the earlier
   "cut #1092 and #1096" decision, which §0.1 shows is not buildable.
3. **Wallet pinning solved by re-anchor** — supersedes the earlier "accept
   pinning" decision (§1.5).
4. **Deploy inert, then activate for design partners**,
   `maxReservationsPerWallet = 1` (§1.3).
5. **Term 12 months**, with `reservationDissolutionDelay` as the buffer and
   a non-zero renewal window forced by the validator (§1.4).
6. **One audit per milestone.**
7. **Branch stays local** — `docs/reservations-spec` not pushed.

## 7. Open questions for review

1. Is a rails release with **no reachable in-kind exit** acceptable in front
   of design partners, given the promise rests on the m2 vault upgrade?
2. **Is the `ReservationVault` proxy-upgradeable?** (§2.2) — if not, m2
   needs a vault re-point, which `feature-spec.md` §15 flags as hazardous.
3. Concrete cap value for `reservationMaxTotalAmount`.
4. Should reservation-eligible **wallets be allowlisted** at activation?
   Depositors pick the designated wallet at reveal, so any Live wallet can
   be selected; an allowlist adds surface no current PR has.
5. Does deploying unreachable-but-audited redemption code count against the
   surface objective? The alternative (intra-PR surgery per §0.1) costs more
   and risks hand-reverting reviewed expiry semantics.

---

## Provenance

Derived 2026-08-21 from `feature-spec.md` (§3-§7, §13, §15, §16),
`epic-merge-plan.md` (§3, §5, §11), `timeline-estimate.md`, and the keep-core
§13 proposal inventory. **Verified against source**, branch-tagged because
the expiry model differs across the stack:
`feat/utxo-reservation-settlement` (#1091) — gate map (:618, :735, :742,
:838, :1083), two-phase acceptance (:401), action timeout (:911), pooled-claim
semantics (:89-91, :807-809), `closeReservation` (:1183-1192), dissolution
proof payload (:891-892), vault gates (:584, :1064), re-anchor authorization
(:718-757), `ReservationProofs.sol` field population (:448-463) and anchor-index
write (:465); `feat/utxo-reservation-backing` (#1093) —
`dissolutionEligibleAt` field and snapshot semantics (:180-186), rewritten
gates (:766, :880), parameter set (:308-314, :1212-1218), mandatory renewal
window (:1232-1236), vault gates (:614, :1133);
`feat/utxo-reservation-guards` (#1094) — `notifyReservationStranded`
(:1363-1378). A scope decomposition for decision, not a commitment of dates.
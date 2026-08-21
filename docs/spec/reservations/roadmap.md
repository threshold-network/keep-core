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
| Dissolution (`requestReservationDissolution`) | permissionless, post-eligibility only (:824, :880; router :302 has no modifier) | **Yes, on a timer** — deferred ~12 months by the term, then permanently open; must be wired (§0.6) |
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

### 0.6 Dissolution is permissionless — leaving it unwired arms a slashing vector

`requestReservationDissolution(uint256)` is `external` with **no modifier**
on the router (`ReservationRouter.sol:302-304`) and no `msg.sender` check in
the library (`Reservation.sol:887-890`, `feat/utxo-reservation-guards`) —
unlike redemption (:634) and renewal (:1153), which are vault-gated. Anyone
can request a dissolution once `block.timestamp >= dissolutionEligibleAt`.

It follows that dissolution **cannot be made unreachable by a minimal
vault**, so it is not deferrable the way redemption and renewal are. Worse,
leaving the keep-core side unwired is not dormancy but a live hazard: if the
wallet does not produce the dissolution Bitcoin transaction before
`reservationActionTimeout`, anyone calls `notifyReservationActionTimeout`
and **the wallet's operators are slashed**. The contract says so directly:
"redemption and dissolution timeouts slash the wallet operators exactly like
a pooled redemption timeout", with `walletMembersIDs` "only consulted for
redemption and dissolution timeouts (the slashing path)"
(`Reservation.sol:961-975`).

So an m1 that ships without keep-core dissolution support hands any
passer-by a **permissionless slashing vector against honest wallets**, armed
automatically the moment the first position passes its dissolution
eligibility date. The term length is therefore not only a promise clock
(§1.4) — it is the deadline by which keep-core dissolution must exist.

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
- **Dissolution** — deferred by the term, not gated: it opens
  permissionlessly at `dissolutionEligibleAt` and must be wired in keep-core
  before then (§0.6).

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
| `reservationTermSeconds` | **12 months** | The promise clock — and the §0.6 slashing deadline (below) |
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

### 1.5 Wallet pinning is solved *during the term*, and only by dissolution after it (revised twice)

The original decision accepted pinning because no unpin mechanism would
ship. That premise is false: **`#1091` ships, so re-anchor ships, and it is
permissionless while the source wallet is `MovingFunds`** (:718-757) —
exactly the retiring-wallet case. A wallet needing to retire can have its
anchors re-anchored to a Live wallet by anyone, dropping its reservation
count so closing can complete.

**But re-anchor expires with the term.** It requires
`block.timestamp < reservation.dissolutionEligibleAt`
(`Reservation.sol:785-788`, `feat/utxo-reservation-guards`). Past
eligibility every movement path is closed except one: redemption and
renewal are vault-gated and unreachable in m1 (§0.2), re-anchor reverts, and
stranding needs the wallet `Terminated`. **Dissolution is the only unpin
left** — the permissionless path of §0.6.

So the two revisions compose into one rule: pinning is solved *inside* the
term by re-anchor and *after* it by dissolution, and m1 must ship both
client-side. If dissolution is unwired (§0.6), a position that passes its
eligibility date pins its custodying wallet **permanently** — wallet closing
requires the reservation count to reach zero, and no path can then decrement
it. That is the same hazard the original decision accepted, merely deferred
by the term length, which is why §0.6 treats keep-core dissolution as
mandatory rather than a nice-to-have.

Residual conditions:
- Both hops need the keep-core executor to sign and prove (§4/§5): re-anchor
  during the term, dissolution after it.
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

### 2.2 m2 needs no Bridge upgrade — but the vault is NOT upgradeable

Because redemption and renewal are vault-gated (§0.2) and their Bridge-side
code ships in m1, enabling them is a **vault-side change**, not a Bridge
storage migration. `epic-merge-plan.md` §11's no-live-action-on-intermediate-
layout rule is therefore satisfied by construction for the redemption
rollout: there is no Bridge layout change at all, and the only m1 in-flight
records are acceptance and re-anchor actions, both transient (bounded by
`reservationActionTimeout`).

**Resolved 2026-08-21: `ReservationVault` is not proxy-upgradeable.**
`contract ReservationVault is IVault, Ownable` — plain `Ownable`, no
`Initializable`, and four `immutable` state variables (`bank`, `tbtcVault`,
`tbtcToken`, `bridge`) set in a `constructor`
(`contracts/vault/ReservationVault.sol:79-142`). `deploy/95_deploy_
reservation_vault.ts:12-17` is a plain `deployments.deploy` with constructor
args and no proxy option. Immutables are baked into bytecode, so a proxy
cannot be retrofitted without refactoring the contract.

So enabling redemption means **replacing** the vault and re-pointing the
Bridge's `reservationVault`. That re-point is **contract-enforced, not
merely discouraged**: `updateReservationParameters` reverts a vault change
unless `reservationTotalAmount == 0` **and** `pendingReservedDeposits == 0`
(`Reservation.sol:1267-1274` on `feat/utxo-reservation-guards`). Nothing is
silently orphaned — the transaction simply fails. (An earlier draft of this
section claimed a swap orphans revealed-but-unaccepted deposits; that is
wrong, and `feature-spec.md` §15 has been corrected to record the guard as
enforced.)

The consequence is worse than orphaning would be, because it is a
**liveness** constraint: the swap is impossible until *every* position has
closed and *every* revealed deposit has been accepted or marked stale. In a
create-only m1 the only position-closing paths are permissionless
dissolution after `dissolutionEligibleAt` (§0.6) and stranding of a
`Terminated` wallet. With a 12-month term (§1.4), the m2 swap therefore
cannot happen until roughly a year after the *last* position was accepted —
and each new acceptance pushes that date out. A vault swap is effectively
unavailable for as long as the product is being used.

**Recommended m1 vault design (avoids the swap entirely):** ship the m1
vault **with** the redemption entry point present but disabled by an
owner-settable flag, so m2 is a single governance transaction
(`setRedemptionsEnabled(true)`) rather than a swap that the guard blocks
indefinitely. This keeps create-only behaviour (unreachable while the flag
is false) and is the same principle already accepted for the Bridge-side
redemption code (§0.2/§1.2). Costs: the vault's redemption plumbing is
deployed and audited in m1, and the flag becomes a governance-safety item
(accidental enable). Given the vault cannot be upgraded *and* the swap is
gated on total quiescence, this is no longer a convenience — **without the
flag, m2's in-kind redemption promise has no reachable delivery path while
positions keep being created.**

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

**m2 order:** enable redemption on the vault — a flag flip under the
recommended design, otherwise a vault redeploy plus `reservationVault`
re-point after draining `pendingReservedDeposits` (§2.2) → then `#1096`
partial (Bridge change, its own audit delta).

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
   acceptance and re-anchor as nonce-carrying proposals, **plus dissolution
   — which is *not* optional.** Dissolution is permissionless (§0.6), so it
   cannot be gated off by a minimal vault, and an unwired dissolution path
   lets anyone slash honest wallets on timeout. Only the redemption proposal
   can stay unwired in m1, because redemption is vault-gated.

## 5. Implementation gaps for m1 (high level)

1. **keep-core two-phase client** — acceptance, re-anchor, **and
   dissolution**, with nonce-carrying proposals, executor duties, and
   regenerated ABI bindings. Dissolution is mandatory per §0.6, and it is
   also the only position-closing path in m1, so it gates both wallet safety
   and §2.2's quiescence. This is the critical path (`feature-spec.md` §16
   item 3).
2. **m1 `ReservationVault`** — exposes the acceptance/credit path, plus the
   redemption entry point **disabled behind an owner-settable flag** per
   §2.2's recommended design (the vault cannot be upgraded, and a swap is
   blocked by the guard until every position closes, so the flag is what
   gives m2 a reachable delivery path). Renewal
   gets no entry point at all.
3. **Anchor-index reconciliation** across `#1091`/`#1094` (§4 items 2-3).
4. **Parameter and activation wiring** — §1.4 values, the deploy-inert
   switch, and governance runbook steps.
5. **Executor duty: re-anchor on rotation** — prompted by
   `WalletMovingFunds`, since §1.5's unpinning depends on it being performed.
   Without this the pinning risk returns in practice.
6. **Monitoring** — anchored wallets (pinning watch), earliest
   `dissolutionEligibleAt` (**both the promise clock and the date the §0.6
   slashing vector arms**), pending dissolution actions approaching timeout,
   pooled-liquidity exposure vs cap.
7. **Tests** — acceptance happy path; timeout and stale-deposit cleanup; cap
   enforcement; a storage-completeness assertion (§2.1); a **reachability
   test** proving redemption and renewal revert for every caller other than
   the vault; and a **dissolution-timeout test** proving the wallet is
   slashed when a permissionless dissolution goes unexecuted (§0.6) — the
   test that justifies wiring it.

## 5.1 Code mass: m1 vs m2, measured (2026-08-21)

Measured from the PR diffs (`gh api .../pulls/N/files`, additions only) and
function-level classification of the four reservation contracts at the m1 tip
(`feat/utxo-reservation-guards`). LoC is a weak proxy for audit cost, but it
is the requested unit and the ratios are informative.

### Shipped code, by milestone

| Bucket | m1 (`#1088`-`#1095`, 7 PRs) | m2 (`#1096`) | Ratio |
|---|---|---|---|
| Production Solidity | **9,206** | **696** | 13 : 1 |
| Solidity tests | 15,896 | 2,441 | 6.5 : 1 |
| Deploy scripts | 432 | 0 | — |
| Docs in-repo | 1,340 | 108 | — |
| Other (ABI JSON, config, lockfiles) | 167 | 14 | — |
| **Total additions** | **27,041** | **3,259** | **8.3 : 1** |

keep-core `#4238` adds 919 production Go + 914 test Go, but models the
pre-two-phase design (§4 item 8), so it is a starting point rather than
shippable m1 content.

Test-to-production ratio on the m1 stack is 1.73:1 — the Solidity is heavily
tested, consistent with `feature-spec.md` §16's "thorough" verdict.

### What the carve-out actually costs

**m1 audits 93% of the feature's production Solidity** (9,206 of 9,902).
Deferring `#1096` removes 696 production lines — about 7%. The carve-out is
achieved by *gating* rather than removing code (§0.2), so almost nothing
leaves the audit surface.

Unreachable-but-deployed logic in m1, by function body (code lines, comments
excluded):

| Path | Functions | Code lines |
|---|---|---|
| Redemption | `requestReservedRedemption` (102), `submitReservedRedemptionProof` (88), `redeemReservation` (38), `retryRedeemReservation` (33), `notifyReservedRedemptionVeto` (31), `resolveLateRedemptionAgainstPending` (23), router wrappers (21) | **336** |
| Renewal | `extendReservation` (49+11), `extendCustody` (29), renewal pause/block/guardian (20) | **109** |
| **Dead weight total** | | **445** |
| Reachable in m1 | acceptance, re-anchor, dissolution, stranding, timeout, caps, params, getters | 1,701 |

So m1 carries **445 code lines it cannot execute — 4.8% of its production
Solidity.** That is the true price of the carve-out, and it is small.

### Effort to prepare m1 for release

Nearly all of it is keep-core Go, not Solidity:

| Item | Est. LoC | Basis |
|---|---|---|
| keep-core two-phase rework (acceptance + re-anchor + **dissolution**) | ~1,400-1,900 prod Go, ~1,000-1,500 test Go | `#4238`'s 919+914 largely rewritten; 3 of 4 action types, nonce-carrying, 6 stubbed `TbtcChain` methods implemented |
| Redemption pause flag on the vault | ~30 prod, ~80 test | Mirrors the shipped `renewalsPaused`/`pauseRenewals` pattern (`ReservationVault.sol:381,409`) |
| Anchor-index reconciliation (`#1091` :465 vs `#1094`) | ~20-60 prod, ~100 test | §4 items 2-3 |
| m1-specific tests (reachability, storage-completeness, dissolution-timeout slashing) | ~200-400 test | §5 item 7 |
| Params, deploy and activation wiring | ~100-200 | §1.3/§1.4 |
| `#1090` rebase | ~0 net | Conflict resolution, no new logic |
| **Total** | **~2,900-4,200** | ~85% keep-core Go |

### Effort to upgrade m1 to m2

| Item | LoC | Note |
|---|---|---|
| `#1096` partial redemption | 3,259 | **Already written** — an open PR, not new work |
| Enable redemption | ~1 tx | `unpauseRedemptions()` under the recommended design; otherwise a blocked vault swap (§2.2) |
| keep-core redemption proposal + partial assembler | ~300-600 prod Go, ~300-500 test Go | The one genuinely new build |
| **New code to write** | **~600-1,100** | Everything else exists |

**Headline:** m1 is ~8x m2 in shipped lines (27.0k vs 3.3k), but the
asymmetry inverts for *remaining* work — m2's Solidity is already written,
so upgrading costs ~0.6-1.1k new lines plus a second audit engagement, while
reaching m1 costs ~2.9-4.2k. The carve-out buys a later deadline, not less
code (`timeline-estimate.md` §6), and it leaves 445 lines deployed but
unreachable to buy that deferral.

## 5.2 What an essentials-only rewrite would cost (2026-08-21)

Asked: if m1 were written from scratch with nothing but essentials, how much
smaller would it be? Estimated per file from §5.1's measurements, with an
explicit keep-factor per file rather than a blended ratio.

**Answer: a third to a half smaller, not an order of magnitude** — and the
reason is the useful part.

### Why the ceiling is low

Redemption and renewal are only **445 of 2,146** function code lines (21%)
in the four reservation contracts. The mass is the **anchor lifecycle** —
two-phase request/proof plumbing, SPV output validation, per-wallet
enumeration, action records and timeout unwinding — and every one of those is
required to create a single reservation. `settleAcceptance` (94),
`unwindPendingAction` (58), `submitReservationProof` (45),
`prepareReservationForSettlement` (42) are not redemption features; they are
what "create" costs.

### Variant A — essentials-only rewrite

Cut redemption and renewal entirely, along with the files that exist only to
serve them (`RedemptionWatchtower.sol` +415 veto integration,
`Redemption.sol` +109 pooled-path hooks), the storage they need
(`reservedRedemptionSettlements`, veto delay, retry credit, renewal window)
and their natspec and tests.

| | Production Solidity | + tests (1.7-1.9x) |
|---|---|---|
| m1 as stacked | 9,206 | ~24,900-26,700 |
| Variant A | **6,171** (-33%) | ~16,700-17,900 |
| Variant A, no router | **5,435** (-41%) | ~14,700-15,800 |

The router line is the single largest swing. `ReservationRouter.sol` (+1,051,
plus ~2,400 test lines in `#1090`) exists only because the full external
surface breaks Bridge's EIP-170 bytecode limit. An essentials-only surface
has roughly 11 entry points instead of 24, so it **may fit in `Bridge`
directly** — worth an hour with the compiler before assuming either way,
because it decides ~3,400 lines.

### Variant B — also drop dissolution, make re-anchor unbounded

The deeper cut is available only when writing from scratch, because it
changes the protocol rather than omitting a PR. Drop dissolution from m1
(~310 function lines, ~910 shipped) and remove re-anchor's
`< dissolutionEligibleAt` gate (§1.5) so wallet rotation never expires.

| | Production Solidity | + tests |
|---|---|---|
| Variant B | **5,261** (-43%) | ~14,200-15,300 |
| Variant B, no router | **4,525** (-51%) | ~12,200-13,100 |

What B buys beyond the lines is worth more than the lines:

- **The §0.6 slashing vector disappears.** No dissolution entry point means
  no permissionless request that slashes a wallet on timeout.
- **The mandatory keep-core duty disappears with it** (~300-500 Go lines),
  leaving acceptance and re-anchor — and re-anchor is the one the wallet
  lifecycle needs anyway.
- **§1.5's pinning cliff disappears.** An unbounded re-anchor means a wallet
  can always rotate its anchors out, at any position age.

What B costs: `reservationTotalAmount` never decrements, so the cap must be
sized for cumulative-ever rather than concurrent usage, and it is raised by
governance when it fills. At design-partner scale (§1.3) that is acceptable;
at production scale it is not, so B is explicitly a launch-posture design
that m2 must replace.

Re-anchor cannot itself be cut: a wallet holding anchors cannot begin closing
until its reservation count reaches zero (§7 guards), so with no re-anchor
and no dissolution, every anchored wallet is pinned for the whole term.

### The cost side of a rewrite

None of this is free, and the comparison is not like-for-like:

- **9,206 lines already exist, reviewed.** They carry 15,896 test lines, a
  finding catalog (`feature-spec.md` §12), 30 findings folded from `#1102`,
  and Mediums found in adversarial re-review. A rewrite discards that and
  re-incurs the review and hardening cycle on new, unreviewed code.
- **The critical path barely moves.** keep-core is unwritten either way
  (§5.1); variant A leaves it unchanged and variant B trims ~300-500 Go
  lines. Since keep-core is the schedule driver
  (`feature-spec.md` §16 item 3), a Solidity rewrite mostly removes work
  that was **not** on the critical path.
- **Audit saving is real but second-order.** Roughly 40% less production
  Solidity is a genuine audit reduction — the strongest argument for a
  rewrite, and the only objective on which it clearly wins.

**Verdict:** a rewrite wins on deployed-and-audited mass (-33% to -51%) and
loses on time-to-first-release, because it trades reviewed code for
unreviewed code without shortening the keep-core path. It is justified only
if audited mass is the dominant objective *and* re-review is genuinely cheap.
The one element worth harvesting regardless of that decision is **variant B's
unbounded re-anchor**, which closes §1.5's pinning cliff in the stacked
design too — see §7 item 6. Note the limit of that harvest: it fixes the
pinning cliff **only**. It does not touch §0.6, because dissolution's
reachability and the timeout-slashing path are independent of re-anchor's
gate, so keep-core dissolution stays mandatory in the stacked design. The
slashing vector disappears only in variant B, where the dissolution entry
point does not exist.

## 6. Decisions confirmed

1. **Create-only m1** — users create only; redemption and renewal exist
   on-chain but are unreachable (§0.2).
2. **Stack ships intact, only `#1096` deferred** — supersedes the earlier
   "cut #1092 and #1096" decision, which §0.1 shows is not buildable.
3. **Wallet pinning solved by re-anchor during the term and dissolution
   after it** — supersedes both the earlier "accept pinning" decision and
   the interim "re-anchor solves it" claim, since re-anchor reverts at
   `dissolutionEligibleAt` (§1.5).
4. **Deploy inert, then activate for design partners**,
   `maxReservationsPerWallet = 1` (§1.3).
5. **Term 12 months**, with `reservationDissolutionDelay` as the buffer and
   a non-zero renewal window forced by the validator (§1.4). The term also
   dates the deadline for keep-core dissolution support (§0.6).
6. **keep-core dissolution ships in m1** — not deferrable, because
   dissolution is permissionless and an unwired path is a slashing vector
   against honest wallets (§0.6).
7. **One audit per milestone.**
8. **Branch stays local** — `docs/reservations-spec` not pushed.

## 7. Open questions for review

1. Is a rails release with **no reachable in-kind exit** acceptable in front
   of design partners, given the promise rests on an m2 governance action?
2. **Approve the flag-gated vault design?** (§2.2) — `ReservationVault` is
   confirmed non-upgradeable (plain `Ownable`, `immutable` constructor
   args), so m2 either flips an owner-settable `redemptionsEnabled` flag on
   the m1 vault, or redeploys and re-points `reservationVault` after
   draining `pendingReservedDeposits` to zero. The flag is recommended; it
   trades a small m1 audit addition for eliminating the swap hazard.
3. Concrete cap value for `reservationMaxTotalAmount`.
4. Should reservation-eligible **wallets be allowlisted** at activation?
   Depositors pick the designated wallet at reveal, so any Live wallet can
   be selected; an allowlist adds surface no current PR has.
5. Does deploying unreachable-but-audited redemption code count against the
   surface objective? The alternative (intra-PR surgery per §0.1) costs more
   and risks hand-reverting reviewed expiry semantics.
6. **Harvest variant B's unbounded re-anchor into the stacked design?** (§5.2)
   Removing re-anchor's `< dissolutionEligibleAt` gate
   (`Reservation.sol:785-788`) would close §1.5's pinning cliff without a
   rewrite. **It does not affect §0.6:** dissolution stays permissionless and
   `notifyReservationActionTimeout` still slashes on non-execution, so
   keep-core dissolution remains mandatory either way. Re-anchor also cannot
   preempt a pending dissolution — it requires `state == Active` (:781) — and
   rotating an eligible position merely moves the execution duty to the
   target wallet, since dissolution reads the current custodian
   (`:904`, gated by `state == Active` and `>= dissolutionEligibleAt` at
   `:895-901`). So this is a pinning fix, not a scope reduction. It is a
   small edit to `#1091`, but it changes reviewed logic in a
   merged-and-folded PR, so it needs a deliberate yes/no rather than a
   drive-by patch.
7. **Is deployed-and-audited mass the dominant objective, or time-to-first-
   release?** (§5.2) A from-scratch essentials rewrite cuts production
   Solidity 33-51% but discards 9,206 reviewed lines plus 15,896 test lines
   and does not shorten the keep-core critical path. The two objectives point
   opposite ways; only one can be optimised.

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
(:1363-1378), permissionless dissolution (:887-890) and its router wrapper
(`ReservationRouter.sol:302-304`, no modifier), the dissolution-timeout
slashing contract (:961-975), and the vault-migration guard (:1267-1274);
`ReservationVault.sol:79-142` plus `deploy/95_deploy_reservation_vault.ts`
for non-upgradeability and the governance activation sequence. A scope
decomposition for decision, not a commitment of dates.
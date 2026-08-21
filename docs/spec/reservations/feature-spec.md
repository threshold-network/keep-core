# UTXO Reservations Feature — Reverse-Engineered Spec

Status: DRAFT — reverse-engineered from 9 open/draft PRs, none merged as of
2026-08-19; **1 of 9 (#1102) merged 2026-08-21** (into `feat/utxo-reservation-core`,
the #1088 branch — see §15). Parameter values below are provisional
(owner-set 2026-08-09, "to be revisited before launch"); nothing is final
until governance sign-off.

## Sources

All PRs are authored by mswilkison, forming one stacked feature
chain implementing a new "UTXO reservation" system for tBTC redemptions:
segregated custody with in-kind (same-coin) redemption, as an alternative to
the existing pooled/shared-UTXO redemption model. **8 of the 9 remain
OPEN/DRAFT; `#1102` merged 2026-08-21** into `feat/utxo-reservation-core`
(the head branch of the stack root `#1088`), folding its 30-review-findings
fix into the stack's base — see merge plan `epic-merge-plan.md` §0.1 for the
verified live inventory.

### keep-core (wallet/signer side, Go)

| # | Title | URL |
|---|---|---|
| 4238 | draft: UTXO reservation wallet-side foundations | https://github.com/threshold-network/keep-core/pull/4238 |

### tbtc-v2 (bridge/contracts side, Solidity) — stacked chain

| # | Title | Branch (base <- head) | URL |
|---|---|---|---|
| 1088 | draft: UTXO reservations — segregated custody with in-kind redemption | (root) `feat/utxo-reservation-core` | https://github.com/threshold-network/tbtc-v2/pull/1088 |
| 1102 | fix: #1088 review follow-ups (backing-invariant + caps) | `feat/utxo-reservation-core` | https://github.com/threshold-network/tbtc-v2/pull/1102 |
| 1090 | feat(bridge): delegatecall reservation router (EIP-170) + RFC 13 | `feat/utxo-reservation-core` <- `feat/utxo-reservation-router` | https://github.com/threshold-network/tbtc-v2/pull/1090 |
| 1091 | feat(bridge): two-phase authorize-then-prove reservation settlement | `feat/utxo-reservation-router` <- `feat/utxo-reservation-settlement` | https://github.com/threshold-network/tbtc-v2/pull/1091 |
| 1092 | feat(bridge): bounded permissionless renewal and strict expiry semantics | `feat/utxo-reservation-settlement` <- `feat/utxo-reservation-renewal` | https://github.com/threshold-network/tbtc-v2/pull/1092 |
| 1093 | feat(bridge): claim-equals-anchor backing model with financed in-kind fees | `feat/utxo-reservation-renewal` <- `feat/utxo-reservation-backing` | https://github.com/threshold-network/tbtc-v2/pull/1093 |
| 1094 | feat(bridge): reveal-side wallet binding, pending-deposit guard, stranding and monitoring | `feat/utxo-reservation-backing` <- `feat/utxo-reservation-guards` | https://github.com/threshold-network/tbtc-v2/pull/1094 |
| 1095 | docs+test: reservation release completeness (M-09) | `docs/utxo-reservation-release` | https://github.com/threshold-network/tbtc-v2/pull/1095 |
| 1096 | feat(reservation): partial reserved redemption (1-in-2-out split) | `docs/utxo-reservation-release` <- `feat/utxo-reservation-partial-redemption` | https://github.com/threshold-network/tbtc-v2/pull/1096 |

Primary design doc read directly (most authoritative source, updated
incrementally by #1090/#1092/#1094/#1096): `docs/rfc/rfc-13.adoc` on tbtc-v2.
Governance parameter doc: `docs/utxo-reservation-frozen-spec.md`. Deployment
  doc: `docs/utxo-reservation-release-runbook.md`.

  *(Provenance split, **updated 2026-08-21**: this previously said that
  #1092-#1096 and keep-core #4238 were grounded "in PR body text / GitHub API
  only, since those branches were not available to verify locally", with a
  local checkout covering only #1088, #1090, #1091 and #1102. That is no longer
  true: every branch in the chain was fetched and read directly during the
  milestone-split inventory pass, and the resulting line-cited rows are in
  `inventory/`. Two caveats replace it. First, the fragments verified against
  `feat/utxo-reservation-guards` predate the #1102 fold, so line numbers in the
  files #1102 touched are pre-fix (`milestone-inventory.md` C-3). Second, the
  frozen-spec and runbook docs cited as primary sources for §10/§11 remain
  unlocatable on any checked branch — treat the §10/§11 parameters as
  PR-body-sourced until verified.)*

Companion analysis: `frost-reservations-interaction.md` investigates this
feature's interaction with the separate FROST/Schnorr threshold-signing migration
(`tlabs-xyz/frost-upgrade`; tbtc-v2 FROST PR chain #971 et al., keep-core #3866/#4005
chains) — not authoritative for this feature's own PRs, but load-bearing context for
§17 below.

---

## 1. Problem and motivation (RFC 13 background)

tBTC's existing redemption path pools every wallet's UTXOs: a depositor's
coin gets swept into the wallet's single main UTXO and redemption pays out
of that shared pool. UTXO reservations add a **segregated custody lane**:
a depositor can have their deposited UTXO held by the wallet without ever
being commingled with the pooled supply, and get it back **in-kind** — with
an unbroken 1-input-1-output lineage — on redemption.

The first implementation (#1088) shipped a **single-phase** lifecycle: every
Bitcoin action (anchor acceptance, reserved redemption, re-anchor,
dissolution) was requested and proven in one step, with all validity checks
performed at proof time against live state. An external security review
found this single-phase model shared one root cause across most of its
findings: *a long-lived, per-position Bitcoin UTXO lifecycle needs a
two-phase, nonce-bound state machine with terminal settlement records* — the
same shape the pooled redemption path already has (`pendingRedemptions` /
`timedOutRedemptions`), generalized to every reservation action. Without it:

- a timeout or veto can race a confirmed Bitcoin redemption, refunding the
  claim on Ethereum after the BTC has irrevocably left custody, exposing the
  honest wallet to an undefeatable fraud challenge (claim double-spend);
- re-anchor and dissolution transactions on Bitcoin have no on-chain
  authorization naming generation/action-type/wallets/fee bound — a
  same-wallet re-anchor is byte-identical to a dissolution, interpretation
  chosen by whoever submits the proof;
- capacity/lifecycle checks performed only at proof time can make an
  already-confirmed Bitcoin spend unprovable;
- the watchtower veto delay was enforced only by the off-chain proposal
  validator, not by the Bridge proof path itself.

PRs #1090-#1096 are RFC 13: the redesign that replaces the single-phase model
with a **two-phase authorize-then-prove settlement machine**, plus a
**reservation router** to solve an EIP-170 (24,576-byte) contract-size
collision, corrected backing/fee mechanics, and wallet-lifecycle integration.

---

## 2. Architecture: the reservation router (#1090)

**Problem.** The mainnet `Bridge` implementation sits within ~400 bytes of
the EIP-170 deployed-bytecode limit even with the optimizer turned down
(measured 24,647 B vs 24,576 B limit before the fix). The reservation
surface does not fit inside `Bridge` alongside everything else.

**Considered and rejected:** an external router contract with privileged
callbacks (used elsewhere for stateless fraud-signature verification)
doesn't work here because reservations mutate core Bridge state (`deposits`,
`registeredWallets`, `spentMainUTXOs`) and exercise Bank authority reserved
for the Bridge address — an external contract would need a wide new set of
privileged Bridge mutators, more bytecode than the refactor removes, plus a
new trusted-party surface.

**Chosen: delegatecall extension.** `Bridge` keeps one storage slot holding
the address of a `ReservationRouter` contract and routes every call with an
unmatched function selector to it via `delegatecall` from its fallback.
Router code executes at the Bridge address, on Bridge storage, with Bridge
Bank authority. The ABI observable at the Bridge address is unchanged;
off-chain clients are unaffected. Result: `ReservationRouter` is 4,245 B;
`Bridge` drops to 22,403 B (runs=100), ~2.1 kB margin.

**Invariants:**
1. **Storage parity.** The router inherits the same storage-bearing bases as
   the Bridge in the same order, declaring exactly one storage variable
   (`BridgeState.Storage self`). New reservation state is appended to
   `BridgeState.Storage` with matching `__gap` reduction. A storage-layout
   parity test compares canonicalized solc layouts of `Bridge`,
   `BridgeStub`, and `ReservationRouter`.
2. **No selector shadowing.** A Bridge-declared selector never reaches the
   router (tested; `Governable` members shared by both are exempt).
3. **No standalone authority.** Direct calls to the router execute on its
   own empty storage — no governance/SPV-maintainer/vault/Bank authority;
   every state-changing entry point on the router itself reverts.
4. **Upgrade model.** `Bridge.setReservationRouter` is one-time and
   governance-gated (idempotent-guarded, reverts if already set). Replacing
   router code after that requires a full Bridge implementation upgrade —
   pointing the delegatecall target at new code *is* an implementation
   change and deliberately carries implementation-upgrade ceremony, not
   parameter-governance ceremony.

New files: `ReservationRouter.sol`, `IReservationBridge.sol`,
`docs/rfc/rfc-13.adoc`. `BridgeState.Storage` gains `address
reservationRouter`.

---

## 3. Core data model

### 3.1 Reservation position (`ReservationRequest`, in `Reservation.sol`)

A **reservation** ("position") is keyed by `reservationKey` (derived from the
originating deposit / anchor UTXO). Key fields (accreted across #1088->#1096):

- `ReservationState state` — `Unknown(0) | Active(1) | ActionPending(2) |
  Closed(3) | Stranded(4)`
- `uint64 mintedAmount` — the outstanding TBTC claim; always equals the
  current anchor value (see §6)
- `bytes20 walletPubKeyHash` — custodying wallet
- `uint32 expiresAt` — UNIX timestamp the custody term expires
- `uint32 dissolutionEligibleAt` — `expiresAt + dissolutionDelay` at the time
  of the last term grant (acceptance or renewal); later governance changes
  to the delay never move an already-granted term's eligibility
- `uint32 termSeconds`, `uint32 gracePeriod` — snapshotted at acceptance (*the #1092 renewal model replaces the `expiresAt + gracePeriod` expiry semantics with `dissolutionEligibleAt` / renewal-window; `reservationGracePeriod` is still a live `updateReservationParameters` arg in reachable source — §10 must say which model survives #1092*)
- `bool retryCredit` — single-use, fee-free redemption-retry entitlement
- `uint64 retryCreditSourceNonce` — generation that minted the outstanding
  retry credit; binds a retry to the exact amount/shape (whole vs partial)
  of that source generation
- `address owner`, anchor UTXO reference, per-wallet enumeration bookkeeping
  (count `walletReservationsCount` plus, on the #1094 line, a swap-remove
  key list `walletReservationKeys`/`walletReservationKeyIndex`), reverse
  anchor lookup `reservationsByAnchorUtxo` (UTXO key -> reservation key,
  **introduced by #1091** and used by `strandReservation` — **corrected
  2026-08-21:** this previously said "#1102 removed it from the merged base,
  so its re-introduction on #1094's line is an open reconciliation". There was
  no removal: the mapping is absent from #1088's branch entirely, so #1102 -
  which merged into that branch - had nothing to remove, and `spentMainUTXOs`
  is a pre-existing Bridge registry rather than a replacement. The real item is
  **two write sites, #1091's and #1094's, and no removal**. See
  `milestone-inventory.md` C-1)

### 3.2 Action generation (`ReservationAction`, added in #1091)

Every Bitcoin-side action against a position is an explicit, nonce-keyed
**generation**: `reservationActions[keccak256(reservationKey, requestNonce)]`.

- `ActionType actionType` — `None(0) | Acceptance(1) | Redemption(2) |
  Reanchor(3) | Dissolution(4)` (partial and whole redemption share
  `Redemption`, distinguished by `isPartial`)
- `ActionState state` — `Unknown(0) | Pending(1) | Settled(2) |
  TimedOut(3) | Vetoed(4) | Superseded(5)`
- `bytes20 targetWalletPubKeyHash`, `uint32 requestedAt`, `uint32 timeoutAt`
- `uint64 txMaxFee` — snapshotted miner-fee cap
- `bool feePaid`, `bool usedRetryCredit`, `uint64 retryCreditSourceNonce`
- `address redeemer`, `uint64 amount` — full claim (whole redemption),
  redeemed portion (partial redemption), reserved deposit value
  (acceptance), or anchor value (otherwise)
- `bytes32 actionDataHash` — keccak256 of the redeemer output script
  (redemptions) or target commitment (re-anchor/dissolution)
- `bytes32 sourceAnchorUtxoHash` — snapshot of the anchor being spent;
  proof requires the transaction's input to match this exactly
- `bool isPartial` — appended at the end of the struct (#1096), no layout
  break; true only for a partial redemption generation
- `uint32 watchtowerDefaultDelay/LevelOneDelay/LevelTwoDelay` — the
  three-level watchtower delay schedule, snapshotted per generation

### 3.3 Deposit-side state

- `PendingReservedDeposit` (`BridgeState.sol`): `isReserved`,
  `walletPubKeyHash` (designated wallet, from the reveal script commitment),
  `refundDeadline`, `refundDeadlineValidated`
- `pendingReservedDeposits` counter — incremented on reveal, decremented on
  acceptance or on being marked stale

### 3.4 Storage layout policy

`BridgeState.Storage` is **append-only**: every new field decrements
`__gap` by exactly the slots added; mappings append freely; nothing is
reordered. Measured per branch (2026-08-21), `__gap` reaches 41 by **two
independent routes that then collide**:

- **core branch**: 48 -> 42 (#1088) -> **41 (#1102, merged 2026-08-21)**.
- **descendant chain** (each branch cut from the pre-#1102 core at 42, so
  these are pre-rebase values): 42 -> **41 (#1090 router)** -> 39
  (#1091 settlement, #1092 renewal) -> 37 (#1093 backing) -> 34
  (#1094 guards, #1095 release) -> 33 (#1096 partial-redemption).

Both #1102 (on core) and #1090 (on its own branch) decrement 42 -> 41 by
different additions. After #1090 (and the stack above it) rebases over the
#1102 fold, the two decrements compete for the same slot budget — the
append-only discipline must be re-verified against the combined diff, not
each PR's own parity test (this is the concrete storage-layout item the
#1090 rebase in §3 step 2 of `epic-merge-plan.md` must resolve, and part of
the §5 audit's 'append-only end-to-end across all 8 PRs' check). The same
discipline applies to the upgradeable `RedemptionWatchtower`
(reservation-generation keys reuse existing veto mappings — no new storage
prefix).

---

## 4. The two-phase settlement machine (#1091, refined by #1092-#1096)

Every generation goes through the same lifecycle:

```
                    request (checks + capacity reserved + snapshot + nonce)
                       |
                       v
                 +-----------+   watchtower delay elapses (redemptions)
                 | Pending   | -------------------------------+
                 +-----------+                                v
                       | veto (redemptions only,       [authorized --
                       | within delay window)           wallet may sign]
                       v                                       |
                 +-----------+                      proof      |   timeout
                 |  Vetoed   | (terminal;         +------------+-----------+
                 +-----------+  proof rejected     v                       v
                               forever)     +-----------+          +-----------+
                                            | Settled   |          | TimedOut  |
                                            +-----------+          +-----------+
                                                                          |
                                                        late proof settles the
                                                        generation: anchor marked
                                                        spent, lineage closed,
                                                        no second refund
```

**Request.** Creates the generation and:
- increments the position's monotonic **request nonce** — state, proofs and
  settlement are keyed by `(reservationKey, nonce)`, so a stale generation
  can never be confused with a newer one;
- performs and **reserves** all capacity/lifecycle checks (global reserved
  total, per-wallet count/amount, target wallet liveness, post-fee dust
  floor, per-wallet main-UTXO action lock) — a wallet that signs an
  authorized action must never find it unprovable because state moved after
  signing;
- **snapshots** every proof/settlement-critical parameter (source anchor
  outpoint hash, tx max fee, redeemer script hash, target wallet, expected
  main-UTXO hash, fee-paid flag, three-level watchtower delay schedule) —
  later governance changes never affect an in-flight generation.

**Authorization.** Redemptions are authorized-for-signing only after the
generation's snapshotted watchtower delay elapses without a veto — enforced
**by the Bridge proof path itself** (a proof is rejected until
`requestedAt + watchtowerDelay <= now` and the generation isn't vetoed). A
wallet that broadcasts early gains nothing: the spend can't finalize before
the guardians' window closes, and if vetoed the spend is unprovable
forever, leaving the signature exposed to fraud-challenge machinery — the
intended punishment for signing unauthorized. Re-anchor and dissolution are
authorized immediately at request time (no watchtower window).

**Settlement** is exactly one of:
- **Proof -> Settled**: SPV proof names `(reservationKey, nonce)`; the
  transaction must match the generation's snapshot exactly (inputs, output
  script/target, fee within bound). Consumed outpoints recorded in
  `spentMainUTXOs`.
- **Timeout -> TimedOut** (terminal, permissionless notification): releases
  reserved capacity + the main-UTXO lock, refunds the escrowed claim
  (redemptions), slashes the wallet like a pooled redemption timeout
  (redemptions), mints the single-use fee-free retry entitlement if the
  generation had paid the fee, returns the position to `Active`.
- **Veto -> Vetoed** (terminal): watchtower detains the escrowed claim
  (penalty/freeze/ban per pooled-parity policy); position returns to
  `Active` (anchor unspent; in-kind claim survives).

**Late proofs** (arriving after a timeout) settle against the `TimedOut`
record's snapshot, not live state: outpoints marked spent, lineage closed,
**no second refund/burn** (the timeout already refunded and slashed — the
residual double-pay is a wallet-fault cost deterred by slashing). If a
*newer pending generation's snapshot also matches the transaction*, the
proof must settle against that generation instead (normal settlement, burn
preserved) — deterministic, economically correct. If a newer pending
generation exists but does **not** match, its escrow is refunded
(unwound) because its anchor no longer exists. A proof against a `Vetoed`
generation is rejected forever.

**Late dissolution + main-UTXO drift.** A dissolution authorized when the
wallet had no main UTXO can race a deposit sweep landing first. The
confirmed dissolution is proven against its record and registered as a
moved-funds sweep request (existing machinery) since its output can't
become the main UTXO. When the snapshot recorded a specific main UTXO,
drift is impossible.

### 4.1 Acceptance

- At **reveal**, a deposit routed to the reservation vault records its
  designated wallet (proven by the reveal script commitment) and increments
  `pendingReservedDeposits`.
- `requestReservationAcceptance` (permissionless) authorizes the anchor:
  checks deposit routing + **designated-wallet binding** (M-05, #1094 —
  must name exactly the deposit's designated wallet), min amount + post-fee
  floor, reserves capacity, snapshots fee bound. Requires at least one
  integer timestamp remain after the deposit-age gate and before both the
  action-timeout and deposit-refund safety margins; authorization ends
  before the exact refund locktime.
- Proof requires the authorization record and the anchor output paying the
  *designated* wallet.
- If the wallet never anchors: authorization times out (capacity released,
  re-requestable within the locktime margin). A stale reserved deposit with
  no live authorization can be permissionlessly marked stale via
  `notifyStaleReservedDeposit` after the refund-deadline margin (never
  while an acceptance authorization is live) — decrements
  `pendingReservedDeposits`; a stale deposit can never be re-authorized (its
  only exit is the depositor's Bitcoin refund).

### 4.2 Redemption — whole and partial (#1091, #1096)

**Whole**: `requestReservedRedemption(reservationKey, redeemer,
redeemerOutputScript, feePaid, useRetryCredit)` redeems the full
`mintedAmount` (1-in-1-out, closes the position on settlement).

**Partial** (#1096): `requestPartialReservedRedemption(reservationKey,
redeemer, redeemerOutputScript, redeemAmount, feePaid, useRetryCredit)`
redeems a `redeemAmount` strictly between the per-tx fee floor and the
whole claim. Both share one internal routine (`_requestReservedRedemption`),
distinguished by `isPartial`.

Bitcoin shape of a partial settlement (1-in-2-out):
- output 0 pays the redeemer script, value `redeemAmount − fee`
  (**bears the entire miner fee**), `fee <= txMaxFee`;
- output 1 re-anchors the remainder to the custodying wallet, value
  **exactly** `anchor − redeemAmount` — claim-equals-anchor holds across
  the split to the satoshi, no foreign sats enter.

On proof: burns the redeemed portion; `mintedAmount`, `anchorAmount`,
`reservationTotalAmount`, `walletReservationsAmount` each drop by
`redeemAmount`; position stays `Active` on the new remainder outpoint
(output index 1). Partials chain (each spends the prior remainder); a whole
redemption closes whatever remains. Guards: `redeemAmount >
reservationTxMaxFee`; `redeemAmount < mintedAmount`; remainder `>
reservationTxMaxFee` (both sides of the split stay above dust).

Timeout/late-proof handling mirrors whole redemption: a timeout returns the
**full** anchor (nothing was redeemed on-chain); a late partial proof
against a matching newer generation must settle it (preserving the burn); a
non-matching newer generation is unwound and refunded, now generalized to
compare shape (`isPartial`), redeemer script, amount, and fee bound.

Redemption requests (whole or partial, paid or retry) are accepted strictly
before expiry (`now < expiresAt`) — see §5.

### 4.3 Re-anchor

`requestReservationReanchor(reservationKey, targetWalletPubKeyHash)` —
allowed only in a migration/approved-rotation context: source wallet in
`MovingFunds`, or caller is governance (rotating a Live wallet, via
`BridgeGovernance.requestReservationReanchor`, forwarded as
`IReservationBridge.requestReservationReanchor`). Target must be Live,
different from source, with capacity (reserved at request). Proof requires
the single output to pay the *recorded* target. Same-wallet re-anchors no
longer exist (removes the earlier re-anchor/dissolution byte-ambiguity — a
proof settles only against the single outstanding generation of its
recorded action type). Anchor shrinks by the financed miner fee (§6).

**Forward compatibility note**: the `targetWalletPubKeyHash` liveness check
(`registeredWallets[...].state == Live`) is already scheme-agnostic — a future FROST/P2TR
wallet registers into the same `bytes20`-keyed mapping via a compatibility public-key hash
(verified directly against the FROST migration branches, not assumed; see §17). The proof
path is not yet scheme-agnostic as written (§17) — a small, identified patch, not a design
gap in this section.

### 4.4 Dissolution

`requestReservationDissolution(reservationKey)` — permissionless once `now >
dissolutionEligibleAt` (snapshotted `expiresAt + dissolutionDelay` at last
term grant), wallet Live or MovingFunds. Acquires the **per-wallet
main-UTXO action lock** (`walletPendingDissolution`): at most one
dissolution per wallet in flight — removes the concurrent-dissolution race.
Snapshots fee bound and expected main UTXO. Bitcoin shape: 1-in (anchor) [+
2nd in: wallet main UTXO] -> 1-out (wallet P2(W)PKH); pool absorbs `anchor −
miner fee`, owner's claim (= anchor) remains outstanding as ordinary TBTC.

---

## 5. Renewal, expiry and the stranding bound (#1092)

Custody term is renewable under a **permissionless, default-allow,
bounded-lookahead** model, replacing the old `expiresAt + gracePeriod`
model:

- A renewal adds **exactly one** current term, possible only inside the
  **renewal window**: `expiresAt - window <= now < expiresAt`, with
  `0 < window < term` enforced atomically at every parameter change and at
  execution. Because window < term, a fresh renewal is immediately outside
  its next window — terms can never stack; max lookahead = one term + one
  window.
- `extendReservation(reservationKey, expectedExpiresAt,
  expectedNewExpiresAt)` — **intent-bound**: commits to the observed
  current expiry and the expected new expiry; stale transactions / mid-flight
  parameter changes revert instead of silently buying a different duration.
  Callable only by `ReservationVault`. On success:
  `expiresAt += term`; `dissolutionEligibleAt = newExpiresAt +
  dissolutionDelay`. Emits `ReservationExtended`.
- `ReservationVault.extendCustody(reservationKey, expectedExpiresAt,
  expectedNewExpiresAt, maxFeeTbtc)` — vault-side owner entry point;
  ownership check, exception-policy check (`!renewalsPaused &&
  !renewalBlocked[key]`), fee bound check, then calls
  `bridge.extendReservation` atomically before the fee transfer.
- **Exception-only policy layer**: a global renewals pause and
  per-reservation blocks, settable by a **renewal guardian** or governance,
  effective immediately; restrictive actions are monotonic (never shorten a
  purchased term, never touch funds). Only governance can unpause, unblock,
  or replace the guardian. A fresh vault starts **paused**.
- Redemption requests (including fee-free retries) accepted strictly before
  expiry (`now < expiresAt`) — no owner action beginning at/after expiry can
  delay dissolution.
- `dissolutionEligibleAt` is **snapshotted per granted term**, so later
  governance changes to `reservationDissolutionDelay` never move the
  eligibility of a term already granted.
- **Stranding bound**: `term + dissolutionDelay + one action timeout`
  (plus, for a governance intervention, one final front-run term:
  detection-to-block latency + window + term + max(delay, redemption
  timeout) + settlement latency) is the hard upper bound on how long a
  residual anchor can pin a wallet.

Protocol hard constants: `MIN_RESERVATION_TERM = 90 days`,
`MAX_RESERVATION_TERM = 730 days`.

### 5.1 Watchtower integration (per-generation)

Veto/objection state is keyed by `keccak256(reservationKey, nonce)`:
objections never accumulate across generations; a once-vetoed reservation
(owner later unbanned) starts every new generation with a clean count. The
watchtower exposes the applicable delay per generation (2h/8h/24h ladder
shared with the pooled path, from `getReservedRedemptionDelaySchedule`); the
Bridge proof path consults it directly (closes H-03); the request path uses
a reservation-scoped safety check (`isSafeReservedRedemption`,
owner/redeemer ban state) rather than the pooled redemption-key check.
`raiseReservedObjection(reservationKey, requestNonce)` is per-generation.

---

## 6. Backing integrity and fee model (#1093, #1096)

**The rule: claim always equals current anchor.** `mintedAmount` (the gross
claim surrenderable at redemption) tracks `anchorAmount` exactly, at every
instant, for every position:

- **Acceptance**: mint = anchor output value; the acceptance miner fee is
  borne by the depositor (like a pooled sweep fee share).
- **Whole redemption**: owner surrenders the claim (= anchor), receives
  `anchor − miner fee` on Bitcoin; supply/backing reconcile exactly, the fee
  is the owner's exit cost.
- **Partial redemption**: claim and anchor both drop by exactly
  `redeemAmount`; the redeemed portion bears its own miner fee on output 0.
- **Re-anchor**: anchor shrinks by the miner fee; the vault **finances** the
  fee from custody-fee revenue (unmints TBTC equal to the fee, burns the
  corresponding Bank balance) — supply shrinks in lockstep with backing;
  rotation costs are borne by the custody fee priced for them, not the
  owner or the peg.
- **Dissolution**: pool absorbs `anchor − miner fee`; owner's claim remains
  outstanding as ordinary TBTC; vault finances the dissolution fee the same
  way. Reconciliation is atomic with the proof.

If the vault's fee reserve can't cover an in-kind fee, **settlement still
proceeds** (a confirmed Bitcoin spend must never fail to settle); the
uncovered remainder becomes a public **`inKindFeeDebtSat`** with a
governance top-up path (`repayInKindFeeDebt(amountSat)`, callable by
anyone). A non-zero debt is a solvency signal, not a stuck settlement. The
global-accounting invariant test asserts debt is zero across normal
lifecycles.

### 6.1 Fee schedule

| Fee | Rate | Notes |
|---|---|---|
| `initiationFeeBps` | 40 bps of gross | 20 bps mint-leg parity + 20 bps first-year custody |
| `extensionFeeBps` | 20 bps of gross | per renewed term |
| `redemptionFeeBps` | 20 bps of gross | pooled parity; not re-charged on wallet-fault retries; a partial charges it on the redeemed portion only |
| `MAX_FEE_BASIS_POINTS` | 500 (hard constant) | sanity cap on any governance fee update |

All-in initiation is 40 bps regardless of horizon (pooled-parity endpoint);
an N-year hold pays `40 + 20(N-1)` bps in initiation+extension fees, plus
  20 bps at redemption = `40 + 20N` bps all-in, vs pooled's flat 40 bps — strictly a
premium at every horizon, by design (the minimum reservation size, not a
fee change, is what's meant to keep carry >= per-position lifecycle cost).
`SATOSHI_MULTIPLIER = 10**10` converts satoshi to 18-decimal TBTC units.

### 6.2 Caps on liability

Because claim == anchor, the sum of anchors is both the asset- and
liability-side total. Caps, all checked/reserved at request/authorization
time (never at proof time):

- `reservationMaxTotalAmount` — global reserved-anchor cap (absolute)
- reserved fraction of total Bank-tracked backing — **not enforced
  on-chain** (see §9); enforced only through the absolute cap above
- `maxReservationsAmountPerWallet` — per-wallet total anchor amount cap
  (`updateReservationCaps`; 0 disables)
- `reservationMaxSingleAmount` — per-position maximum (`updateReservationCaps`;
  0 disables)
- `maxReservationsPerWallet` — per-wallet reservation **count** cap

---

## 7. Wallet lifecycle integration and guards (#1094)

- **M-05 — designated-wallet binding**: a deposit revealed to the
  reservation vault records its designated wallet (from the reveal's script
  commitment); `requestReservationAcceptance` must name exactly that
  wallet. A Byzantine wallet can no longer force custody onto an unrelated
  Live wallet.
- **M-04 — pending-deposit tracking + vault-migration guard**: revealed
  reserved deposits count as pending until accepted or marked stale.
  `updateReservationParameters` refuses a `reservationVault` change while
  either active anchors or pending deposits exist (closes the "old-vault
  deposits become pool-sweepable while old vault still mints on sweep
  callback" scenario). A stale deposit's only exit is the depositor's
  Bitcoin refund; it can never be re-authorized.
- **H-06 — stranding on termination**: `notifyReservationStranded`
  (permissionless, wallet must be `Terminated`): position -> `Stranded`,
  capacity released, any pending action unwound (pending redemption escrow
  returns to its redeemer), enumeration/anchor-index cleared. The anchor is
  deliberately **not** marked honestly spent (a terminated-wallet spend
  stays recognizable as such). *(There is a second, distinct entry into
  `Stranded` that this spec must also record: on the dissolution-*proof*
  path, `ReservationProofs` flips `state = Stranded` whenever the custodying
  wallet is `Terminated` after `closeReservation` — there the dissolution tx
  is proven and its outpoints recorded, so the "not honestly spent"
  property does NOT hold on that path. And the standalone
  `notifyReservationStranded` primitive is traced to #1094 PR-body text, not
  yet read from a diff — verify.)* Owner keeps minted TBTC as an ordinary
  pooled claim — backing shortfall socialized like a terminated wallet's
  main UTXO today. **The depositor's tBTC balance never changes** — stranding
  removes the in-kind redemption *option*, not the claim; it is a
  bookkeeping + evidence transition. `Terminated` has three wallet-lifecycle
  causes (moving-funds timeout, moved-funds-sweep timeout,
  fraud-challenge-defeat timeout); the two timeout paths are liveness
  failures and may leave the BTC intact but unreachable by protocol policy,
  so a `Stranded` reservation is not evidence of theft. `ReservationStranded(key, wallet, owner, amount)` event
  is the evidence hook for a future governance compensation module
  (interface deliberately not yet stubbed in storage).
- **L-01 — monitoring surface**: per-wallet reservation enumeration
  (`walletReservationsCount` + the #1094-line `walletReservationKeys`
  swap-remove list), reverse anchor lookup (`reservationsByAnchorUtxo` —
  introduced by #1091, written again by #1094; **corrected 2026-08-21**, it
  was not "removed from the merged base by #1102" - two write sites, no
  removal, see `milestone-inventory.md` C-1), per-wallet count/amount getters,
  pending-deposit getters.
- **Wallet closing guard**: a wallet holding reservation anchors (or
  pending reservation actions) cannot *begin* closing — moving-funds
  completion requires the reservation count to be zero (existing
  finalize-closing guard stays as defense in depth). Voluntary closure is
  blocked at the settlement layer: a wallet holding anchors must retire
  through `MovingFunds` and drain them first.
- Reserved redemption requests require the wallet to be Live or
  MovingFunds; every settlement path (timeout, veto, proof) works in any
  wallet state.
- `Wallets.sol`: closing preconditions now also require
  `pendingMovedFundsSweepRequestsCount == 0`; `notifyWalletMovedFundsSweepTimeout`
  accepts Closing/Closed states without slashing; new
  `rearmMovingFundsTimeout`; `movingFundsRequestedAt = 0` marks a proven
  generation's completion (zero can't be reported as a timeout). A Closing
  wallet reactivates to MovingFunds on a sweep proof.

---

## 8. Release completeness clarifications (#1095, docs-only, M-09)

No new contract logic. Documents:
- the two-phase lifecycle with **snapshotted** watchtower delay, redemption
  delay, and dissolution-bound stranding timeouts, for both Live-custody
  wallets (two action timeouts: MovingFunds then terminate) and
  MovingFunds wallets (one action timeout); Live-source rotation needs
  governance;
- an off-chain governance sign-off ledger for frozen parameters (the
  `utxo-reservation-frozen-spec.md` doc, §10 below);
- the deploy-inert-then-activate release runbook (§11 below);
- a Medium found+fixed during adversarial re-review — an "acceptance
  late-settlement reserved-capacity leak" — committed to #1094, regression
  test landed on this PR's tip.

---

## 9. Explicit accepted limitation: reserved-fraction target is off-chain

The design's "reserved backing <= 10-20% of total" ceiling is **not an
on-chain check**: the Bank exposes no trustless aggregate backing figure,
and introducing an oracle to gate a throttle is judged a worse risk than
the throttle protects against. Instead:

- `reservationMaxTotalAmount` is the sole on-chain lever.
- Governance sets it to at most the target fraction of the current total
  BTC backing (observed off-chain) and re-tightens it as backing moves.
- Launch cap is deliberately conservative (100 BTC, see §10), below the
  10% target at any plausible launch backing, ramping up as the feature
  proves out.

Flagged in the frozen-spec doc as an explicit accepted limitation for
audit.

---

## 10. Governance parameter surface (frozen-spec doc, as of #1096)

Every value is provisional (owner-set 2026-08-09) pending final governance
sign-off; mechanics are fixed, numbers are not.

### Bridge reservation parameters (`updateReservationParameters`)

| Parameter | Meaning | Provisional launch value | Bounds / notes |
|---|---|---|---|
| `reservationVault` | Liability-side vault | deployed vault | Changeable only with zero active reservations **and** zero pending reserved deposits |
| `reservationMinAmount` | Minimum anchor amount | 10 BTC (1,000,000,000 sat) | Must exceed `reservationTxMaxFee` |
| `reservationTxMaxFee` | Per-tx Bitcoin miner-fee cap | 50,000 sat (0.0005 BTC) *(pending, fee-market)* | > 0; a partial's redeemed portion and remainder must each exceed it |
| `reservationDissolutionTxMaxFee` | Dissolution-tx miner-fee cap (2-in-1-out shape) | *(pending — same fee-market basis as `reservationTxMaxFee`)* | Must be > 0 (`reservationTxMaxFee` covers acceptance/redemption/re-anchor 1-in-1-out and partial 1-in-2-out) |
| `maxCumulativeReanchorFee` | Cumulative re-anchor miner-fee cap, per reservation | *(pending — the fee-grinding bound)* | Must be > 0; per-reservation `cumulativeReanchorFee` must stay under it (see §15 fee-grinding) |
| `reservationTermSeconds` | Custody term per acceptance/renewal | 365 days | Hard bounds 90-730 days (protocol constants) |
| `reservationDissolutionDelay` | Post-expiry delay before dissolvable | 7 days | Snapshotted per granted term |
| `reservationMaxTotalAmount` | Global reserved-anchor cap | 100 BTC (10,000,000,000 sat) | Deliberately conservative; below the 10% fraction target at plausible launch backing |
| `maxReservationsPerWallet` | Per-wallet reservation count cap | 10 | Binds fully only if the amount cap is disabled — with `reservationMinAmount`=10 BTC and `maxReservationsAmountPerWallet`=50 BTC the amount cap already limits a wallet to 5 positions (*pending #1093 verification of the amount cap*); still bounds re-anchor ceremonies per rotation window |
| `reservationActionTimeout` | Timeout for acceptance/re-anchor/dissolution | 48 hours | Must be **> 2 hours**; acceptance needs `> 4 hours` in practice (see note below) |
| `reservationRenewalWindowSeconds` | Renewal window before expiry | 30 days | `0 < window < term`, enforced atomically |

Acceptance timing note: the proposal validator requires the deposit be
older than 2 hours (`DEPOSIT_MIN_AGE`) while preserving the final 2-hour
action-timeout safety margin (`REQUEST_TIMEOUT_SAFETY_MARGIN`); an
acceptance requested immediately after reveal needs
`reservationActionTimeout > 4 hours` to have any valid window, and signing
  must begin at least `DEPOSIT_REFUND_SAFETY_MARGIN = 24 hours` before the
  deposit's Bitcoin refund locktime (the margin is a guard-railing subtrahend
  on signing *start*, not a window the whole authorization must fit inside).

### Bridge reservation caps (`updateReservationCaps`)

| Parameter | Meaning | Provisional launch value |
|---|---|---|
| `maxReservationsAmountPerWallet` | Per-wallet total anchor amount | 50 BTC (5,000,000,000 sat) (0 disables) |
| `reservationMaxSingleAmount` | Single-reservation maximum | 25 BTC (2,500,000,000 sat) (0 disables) |

*Provenance (verify before relying): `maxReservationsAmountPerWallet` and
`reservationMaxSingleAmount` (both `0 disables`) and the
`updateReservationCaps` entry point are attributed to #1093's H-04 caps
rework but are NOT present in the reachable checkouts (#1093 not available
here) — only global `reservationMaxTotalAmount` and per-wallet count
`maxReservationsPerWallet` are on-chain-verified. Treat this table as
PR-body-sourced until #1093 is confirmed; the exposure math in
`shortfall-design-space.md` §2 and `stranding-compensation-proposal.md` §3
depends on the amount cap.*

### `ReservationVault` fees and reserve

| Parameter | Value | Notes |
|---|---|---|
| `initiationFeeBps` | 40 | fixed constant |
| `extensionFeeBps` | 20 | fixed constant |
| `redemptionFeeBps` | 20 | fixed constant |
| `MAX_FEE_BASIS_POINTS` | 500 | fixed constant |
| `feeReserveTarget` | *(pending — seed before unpausing)* | excess sweeps to treasury via `sweepFees` |

### Open economics items (out of contract scope)

- Pricing of the embedded senior-liquidity option (a reserved owner holds a
  demand claim against term-locked backing) — financial-modeling task,
  tracked, not a contract parameter.
- `updateFees` has no governance-delay wrapper yet; acceptance has no
  user-facing slippage bound (renewal/redemption do, via `maxFeeTbtc`) since
  it's wallet-initiated. Tracked follow-ups, don't block the settlement
  audit.
- No grace penalty by design — the post-expiry delay is a settlement
  coordination window, not an owner grace period.
- Two governance-set relationships the contracts do **not** enforce, flagged
  from the adversarial re-review: (1) keep
  `reservedRedemptionVetoDelay < redemptionTimeout` when tuning either
  (else a redemption could time out before its veto window closes); (2)
  avoid re-pointing `reservationVault` while any reserved deposit could
  still settle late (blocked on-chain while positions/pending deposits
  exist, but late-acceptance settlement was hardened to credit the
  deposit's immutable revealed vault as extra defense).

---

## 11. Deployment sequencing (release runbook)

**Overriding rule**: reservations must never activate on a temporary
storage layout that later needs live-state migration. Deploy inert, then
activate last; ship the complete Bridge/router/watchtower stack as one
coordinated release.

1. **Bridge implementation upgrade** carrying the full reservation storage
   append (all stacked PRs at once, not piecemeal). Run storage-layout
   parity tests before submitting.
2. **Deploy libraries** (`Reservation` linking `ReservationProofs`) and
   **`ReservationRouter`**. Complete the **BridgeGovernance replacement**
   (below) before continuing — the incumbent doesn't expose the router
   forwarder.
3. **Wire the router**: `BridgeGovernance.setReservationRouter(router)`.
   Until set, every reservation selector reverts with `"Unknown
   function"`; pooled Bridge unaffected. Vault remains untrusted through
   this and all following steps until the final activation transaction.
4. **Watchtower implementation upgrade** (per-generation reserved
   objections + request-time delay snapshots) — must complete before vault
   activation.
5. **Deploy `ReservationVault`** — deploys with renewals **paused**,
   ownership transferred to governance immediately.
6. **Deploy `MaintainerProxyV2`** — the legacy immutable mainnet proxy
   doesn't expose `submitReservationProof`. V2 copies the legacy SPV-
   maintainer allowlist, transfers ownership to governance; governance must
   authorize it on both Bridge and `ReimbursementPool`. Legacy proxy stays
   authorized through client cutover.
7. **Redeploy `WalletProposalValidator`** against the upgraded Bridge;
   repoint coordinator/maintainer config.
8. **Configuration and activation (governance, last)**:
   1. Confirm `Bridge.isVaultTrusted(vault) == false`.
   2. `beginReservationParametersUpdate` -> finalize (after delay) — sets
      `reservationVault` + launch parameters.
   3. `beginReservationCapsUpdate` -> finalize — sets launch amount caps.
   4. `ReservationVault.updateFeeReserveTarget(target)` — seed the in-kind
      fee reserve.
   5. `ReservationVault.setRenewalGuardian(guardian)` — optional.
   6. `ReservationVault.unpauseRenewals()` — if renewals available at
      launch (affects only `extendCustody`, not deposit reveal/acceptance).
   7. `BridgeGovernance.setVaultStatus(vault, true)` — **the sole final
      activation gate**. Trusting the fully configured vault permits deposit
      reveals to it and opens the reservation lane.

`BridgeGovernance` is **non-upgradeable**; adopting the two immediate
forwarders (`setReservationRouter`, governance-approved
`requestReservationReanchor`) and two delayed begin/finalize flows
(`...ReservationParametersUpdate`, `...ReservationCapsUpdate`) requires
deploying a new `BridgeGovernance` instance and transferring Bridge
governance to it via a standard incumbent-governance-delay handoff
(`beginBridgeGovernanceTransfer` -> wait delay -> `finalizeBridgeGovernanceTransfer`),
with the new instance's ownership transferred to the council/timelock
*before* being proposed as Bridge governance (never finalize while the
deployer still owns the new instance).

### Pre-audit checklist (from the runbook)

- Full Solidity suite green; Slither (CI-pinned 0.9.0) 0 results.
- `Bridge` deployed bytecode under EIP-170 with safety margin; router and
  libraries each under EIP-170.
- Storage-layout diff append-only and expected (parity test green).
- Fork dry-run of the full activation sequence; verify `MaintainerProxyV2`
  authorized on Bridge + pool with expected SPV maintainers, vault left
  untrusted/inert until the final `setVaultStatus` transaction.
- Frozen-spec parameter values signed off by governance.
- keep-core executor updated for the two-phase ABI, or explicitly
  out-of-scope for the audit with the feature deployed disabled.
- Re-review confirms the settlement-class findings are resolved.

---

## 12. Review findings closed, by PR

| Finding | Description | Closed in |
|---|---|---|
| H-08 | (from #1088's original review) | #1088 |
| Redemption underflow | Underflow-safe redemption check | #1088 |
| C-01 | Late-proof settlement could double-refund | #1091 |
| H-01 | Re-anchor/dissolution byte-ambiguity, no on-chain authorization | #1091 |
| H-02 | Proof-time-only validation vs live state (signed tx could become unprovable) | #1091 |
| H-03 | Watchtower veto delay enforced only off-chain, not by proof path | #1091 |
| H-05 (request side) | Lifecycle gating (wallet state, timing) not enforced at request | #1091 |
| H-07 | Concurrent-dissolution race | #1091 |
| M-01 | Snapshot policy for term/fee/timeout parameters | #1091 |
| M-02 | Retry-entitlement lifecycle (mint/consume/restore) | #1091 |
| M-03 | Watchtower objection state not scoped per generation | #1091 |
| H-04 | Term-stacking via renewal | #1092 |
| M-06 | Mid-flight governance parameter changes vs in-flight renewal | #1092 |
| M-09 | Grace-period governance rollback could retroactively move eligibility | #1092 (mechanics) / #1095 (docs+tests) |
| H-04 (backing) | Dissolution permanently underbacks by cumulative in-kind fees; re-anchor temporarily underbacks | #1093 |
| M-06 (caps) | Caps didn't match the design (liability vs. anchor accounting) | #1093 |
| H-06 | Voluntary/involuntary termination could strand live anchors with no recovery path | #1094 |
| M-04 | Vault change could orphan already-revealed reserved deposits | #1094 |
| M-05 | Anchor not cryptographically bound to the wallet the deposit was revealed for | #1094 |
| L-01 | No monitoring/enumeration surface for reservations | #1094 |
| (unlabeled, re-review) | Acceptance late-settlement reserved-capacity leak | fixed in #1094, regression test on #1095's tip |

Note: `H-04`/`M-06` are reused labels across two different review rounds
(the original #1088 review vs. the settlement-rework re-review) — the table
disambiguates by context. `L-01` also appears twice (see keep-core section
below is unaffected; both L-01 uses are tbtc-v2-side).

---

## 13. keep-core: wallet/signer-side status (#4238)

Standalone draft PR (not stacked), base `main`. Gated on the tbtc-v2 Bridge
ABI being published (needs #1088+ merged and the npm package regenerated) —
Ethereum bindings and coordination-executor wiring are **deliberately
deferred** to a follow-up.

**What ships in #4238:**
- `pkg/tbtc/reservation.go` (new, 729 lines): `Reservation` /
  `ReservationStatus` / `ReservationParameters` types; four
  `CoordinationProposal` implementations (`AnchorReservationProposal`,
  `ReservedRedemptionProposal`, `ReAnchorReservationProposal`,
  `DissolveReservationProposal`); four unsigned Bitcoin transaction
  assemblers (one per action shape).
- `pkg/tbtc/wallet.go`: four new `WalletActionType` values appended after
  `ActionMovedFundsSweep` (`ActionReserveAnchor=6`,
  `ActionReservedRedemption=7`, `ActionReAnchor=8`, `ActionDissolve=9`) —
  appended, not inserted, to preserve serialized compatibility.
- `pkg/tbtc/chain.go`: six new `TbtcChain` interface methods
  (`GetReservation`, `ReservationParameters`,
  `ValidateReserveAnchorProposal`, `ValidateReservedRedemptionProposal`,
  `ValidateReAnchorReservationProposal`,
  `ValidateDissolveReservationProposal`).
- `pkg/chain/ethereum/tbtc.go`: the six methods above **stubbed**,
  returning descriptive errors until the Bridge ABI is regenerated.
- `pkg/clientinfo/performance.go`, `pkg/tbtc/marshaling.go`: metric names
  and the coordination-proposal unmarshal factory extended for the four
  new action types.
- Carries an unrelated one-line fix (separately labeled commit, cherry-
  pickable to `main`) for a pre-existing break in
  `pkg/tbtcpg/redemptions.go:225` (old 8-tuple vs. new struct return from
  `GetRedemptionParameters()`).

**What's explicitly deferred** (per #4238 and the runbook's keep-core
follow-up section, gated on the two-phase ABI landing in #1091+):
- Ethereum bindings (blocked on published npm typechain artifacts).
- Coordination executor + `tbtcpg` proposal generation wiring.
- **Two-phase awareness**: proposals must carry a request nonce; the
  coordinator must call the on-chain request function first
  (`requestReservationAcceptance` / vault redemption entry points /
  `requestReservationReanchor` / `requestReservationDissolution`), read
  back the generation, and only then schedule signing. Proofs submit with
  `(reservationKey, requestNonce)`.
- The executor must respect the on-chain watchtower-delay gate (never sign
  a redemption generation before its delay elapses), prioritize valid
  pre-expiry reserved redemptions, drive expired positions toward
  dissolution after pending actions resolve, and never propose dissolution
  before the snapshotted `dissolutionEligibleAt`.
- **On `Live -> MovingFunds`, re-anchor every open reservation to a Live
  target** (`requestReservationReanchor`, permitted for `MovingFunds`
  source wallets, §4.3). A rotating wallet's un-re-anchored anchors are
  stranded if `movingFundsTimeout` fires — re-anchor is the intended
  migration path, and leaving its initiation unspecified turns every routine
  wallet rotation into a stranding candidate. Target choice, capacity
  reservation (reserved at request), and fee handling are the executor's
  responsibility; it should initiate re-anchor promptly on
  `WalletMovingFunds(walletPubKeyHash)`, not wait for an expiry signal.
- **Watch for `Terminated` wallets still custodying un-stranded
  reservations and call `notifyReservationStranded` for each**
  (permissionless, §7 H-06). Releases the dead wallet's reserved capacity
  and emits the recovery-evidence event; until it fires, a terminated
  wallet's anchors still count against the global `reservationMaxTotalAmount`
  cap. Termination is triggerable by any of three wallet-lifecycle paths
  (moving-funds timeout, moved-funds sweep timeout, fraud-challenge defeat
  timeout), of which the timeout paths are liveness failures requiring no
  malice — see `exit/stranded.md` §3.4 for the operative
  framing.
- Monitoring should watch `pendingReservedDeposits`, `inKindFeeDebtSat`,
  dissolution-eligible positions, per-wallet reserved amount/count, and
  terminated wallets holding un-stranded reservations (the executor's
  `notifyReservationStranded` duty above), via
  the new getters.
- Protobuf marshaling (proposals currently JSON; TODO markers present —
  requires new message types in `pkg/tbtc/gen/pb/message.proto`).
- Integration tests (unit tests only cover action parsing, proposal
  marshaling roundtrips, assembler input validation).

**Note on #4238's own transaction shapes**: as drafted, #4238 models the
lifecycle as it stood at #1088 (single-phase, whole redemption only, no
partial-redemption 1-in-2-out shape). It predates and has not yet
incorporated the two-phase (#1091), renewal (#1092), backing (#1093),
guards (#1094), or partial-redemption (#1096) redesigns — this is the exact
gap the runbook's keep-core follow-up section calls out.

---

## 14. Cross-cutting invariants (consolidated)

1. **Claim ≡ anchor**, always, for every position (§6).
2. **1-in-1-out lineage** for acceptance, whole redemption, and re-anchor;
   **1-in-2-out** for partial redemption (redeemer + remainder); dissolution
   spends the anchor optionally alongside the wallet's main UTXO.
3. **Segregated custody**: a reserved deposit is never swept into the
   pooled main UTXO (`DepositSweep` rejects deposits with
   `pendingReservedDeposit.isReserved == true`).
4. **Nonce-bound generations**: `(reservationKey, requestNonce)` uniquely
   identifies every action attempt; stale generations can never be
   confused with newer ones.
5. **Snapshot-at-request**: every proof/settlement-critical parameter is
   fixed at request time; later governance changes never affect an
   in-flight generation.
6. **No second refund**: a late proof against a `TimedOut` record never
   re-triggers a Bank movement.
7. **Vetoed is terminal and unprovable forever.**
8. **Non-stacking renewals**: `renewalWindow < term` makes stacking
   arithmetically impossible.
9. **Dissolution eligibility is snapshotted per term grant** — never
   retroactively movable by governance.
10. **Wallet closing is blocked** while it holds any reservation anchor or
    pending reservation action.
11. **In-kind fees are financed, never silently leaked** — shortfalls become
    public, repayable debt.
12. **Storage is append-only** — `BridgeState.Storage.__gap` shrinks by
    exactly the slots added, on both `Bridge` and `ReservationRouter`.
13. **Router has no standalone authority** — it only ever executes
    meaningfully via `Bridge`'s delegatecall fallback.

---

## 15. Open questions / risks (consolidated)

- **Decision & loss-story anchor (2026-08-21)** — the accepted fallback is
  `Stranded`; the emergency-exit Mechanism 1 is deferred as reference
  (Decision block: `exit/README.md`). The loss-story docs are
  `shortfall-design-space.md` (who-pays: Space A and Space B rejected; Space
  C viable only conditional on an unbuilt `anchorAmount`/`mintedAmount`
  decoupling — **not adopted**) and `stranding-compensation-proposal.md`
  (Tiers 0-1 are the decided build; Tier 0 is the stranding-frequency
  evidence instrument). The compensation-module items below are answered
  there, not unowned gaps.
- **Parameter sign-off**: every governance value in §10 is provisional
  (owner-set 2026-08-09) and requires final governance sign-off before
  launch; four values (`reservationTxMaxFee`, `feeReserveTarget`,
  `reservationDissolutionTxMaxFee`, `maxCumulativeReanchorFee`) have no
  proposed number yet at all.
- **Reserved-fraction target is an off-chain governance rule**, not an
  on-chain invariant — accepted limitation, flagged for audit (§9).
- **Senior-liquidity option pricing** (whether 20 bps/yr correctly prices
  the embedded option) is an open financial-modeling question, out of
  contract scope.
- **`updateFees` governance-delay wrapper** and a per-position
  initiation-fee snapshot are tracked follow-ups, not yet implemented.
- **Governance-set relationship gap not enforced on-chain**: keep
  `reservedRedemptionVetoDelay < redemptionTimeout` (no relational check
  exists in `updateReservationParameters`).
- **Vault re-pointing is contract-enforced, and the vault is not
  upgradeable** (verified 2026-08-21). `updateReservationParameters` reverts
  a `reservationVault` change unless `reservationTotalAmount == 0` **and**
  `pendingReservedDeposits == 0` (`Reservation.sol:1267-1274` on
  `feat/utxo-reservation-guards`), so a re-point cannot silently orphan
  positions or revealed deposits — it simply reverts. But
  `ReservationVault` is a plain `Ownable` contract with four `immutable`
  constructor args and no `Initializable`
  (`contracts/vault/ReservationVault.sol:79-142`; deployed by a bare
  `deployments.deploy`, `deploy/95_deploy_reservation_vault.ts:12-17`), so
  changing vault behaviour requires a **redeploy plus a re-point that is
  blocked until every position closes and every revealed deposit is
  accepted or marked stale**. Any staged rollout that plans to change vault
  behaviour later should instead ship the behaviour switchable inside the m1
  vault — see `roadmap.md` §2.2.
- **Governance compensation module for `Stranded` positions** is stubbed
  only as an event (`ReservationStranded`), no storage/interface yet —
  designed in `stranding-compensation-proposal.md` (Tiers 0-1, the decided
  build) per the Decision in `exit/README.md`.
- **keep-core is two redesign generations behind** the current tbtc-v2
  design (§13) — landing keep-core support is explicitly gated on the
  tbtc-v2 ABI publishing, itself gated on #1088-#1096 merging. The required
  second keep-core PR is **still not open as of 2026-08-21** (verified in
  the merge-plan inventory: `#4238` is the only keep-core PR targeting
  `reservations-epic`), so keep-core support is a hard hold, not a pending
  review. Verified
  directly: `pkg/tbtc/gen/pb/` on the keep-core PR branch has no
  reservation message types (JSON marshaling only, as the PR's own TODOs
  say), and the six new `TbtcChain` Ethereum-binding methods are stubs
  that return errors, not implementations.
- **Re-anchor fee grinding: capped, but the cap is unbounded as a ratio.**
  Resolved at the source in #1088: `maxCumulativeReanchorFee`
  (governance-set), per-reservation `cumulativeReanchorFee` tracking, and a
  re-anchor dust floor (`pr-review-followups.md` item 5 — also
  claimed by #1093's H-04 backing fix; confirm the two caps are compatible,
  not stacked). What remains open is the *bound itself*: backing left after
  maximal grinding is a constant `reservationTxMaxFee + 1 -
  reservationDissolutionTxMaxFee` independent of claim size; because
  `maxCumulativeReanchorFee` caps the grind on large claims, fractional
  underbacking peaks where the budget and the dust floor bind simultaneously,
  at claim = `maxCumulativeReanchorFee + reservationTxMaxFee + 1` (~99.5%
  loss at fixture values), and improves above that. No expression on the fee
  caps alone can deliver a ratio guarantee
  (followups item 7). Governance must pick among four levers (relational
  `require` / per-position fraction of `mintedAmount` / proportional dust
  floor / leave unbounded with monitoring — *`ReservationDissolved` exposes
  realized fee loss per dissolution on the `#1102` line now (merged 2026-08-21
  into `feat/utxo-reservation-core`); the `#1091+` event carries no fee
  fields, so a monitor on the merged base has the fee-bearing variant*). Decision
  deferred to the #1093 backing review.
- **Stranding reachability from all three termination paths: verify,
  don't assume.** H-06's fix requires wallet `Terminated`, which the
  moving-funds / moved-funds-sweep / fraud-challenge-defeat timeout paths
  all reach — but `walletReservationsCount == 0` is checked only on the
  graceful closing path, and naively porting it onto the punitive paths
  would be griefable (a dust reservation blocking deserved slashing).
  Confirm `notifyReservationStranded` is reachable and un-griefable on all
  three paths, not only the graceful one (followups item 1).
- **Re-anchor/dissolution proof-submission gating after the authorize/prove
  split: verify.** `requestReservationDissolution` / `requestReservationReanchor`
  are permissionless on the request side, but whether the submit-proof
  entry points remain SPV-maintainer-gated (no parallel of
  `notifyRedemptionTimeout` for these) is unconfirmed in the split. If still
  maintainer-gated, an SPV-maintainer stall blocks dissolution with no
  permissionless fallback (followups item 3).
- **Vault rotation can be blocked by a single active owner (governance-
  liveness cost).** The vault-change guard is `reservationTotalAmount != 0` —
  any one reservation owner who never redeems can block rotation
  indefinitely. M-04's #1094 fix covers pending-deposit safety, not the
  already-Active blocking facet (followups item 2). Accepted tradeoff or
  needs policy — verify #1094 doesn't rely on the pending-only guard.
- **No live-state migration path exists** for any live `ReservationAction`
  record written by an intermediate implementation — the runbook's
  deploy-inert-then-activate-last sequencing exists specifically to avoid
  ever needing one; do not deploy the stack piecemeal.
- **Referenced findings doc does not exist.** The release runbook's own
  header cites `docs/utxo-reservation-review-findings.md` as a companion
  ("the closed findings"). Checked directly on
  `feat/utxo-reservation-guards`, `docs/utxo-reservation-release`, and
  `feat/utxo-reservation-partial-redemption` (2026-08-19): the file is
  absent on all three. The finding-code-to-PR mapping in §12 is
  reconstructed from PR bodies, not sourced from the authoritative doc the
  team's own runbook points to.
- **Pre-audit checklist is entirely unchecked.** Every box in the
  runbook's checklist (§11) and the frozen-spec sign-off ledger (§10) is
  still `[ ]`/`☐`, including "Slither clean," "fork dry-run of the full
  activation sequence," and "re-review confirms the settlement class is
  resolved." No external/third-party audit has been engaged yet — the
  closed findings are from internal adversarial self-review rounds only.
- **Stack branches are behind their bases.** `gh pr view` on #1094 and
  #1096 reported `Merge state: BEHIND` (checked 2026-08-19) — normal for
  active stacked work, but real rebase/conflict risk sits between here and
  a mergeable stack. That risk has since materialized: **#1090 is
  CONFLICTING with its base as of 2026-08-21**, a mechanical result of
  `#1102` merging into `feat/utxo-reservation-core` (the branch #1090 is cut
  from) — `feat/utxo-reservation-router` needs a rebase over the `#1102`
  fold before the stack can merge (§3 step 2 of `epic-merge-plan.md`).
- **`__gap` reaches 41 by two independent decrements — rebase must
  reconcile the slot budget (§3.4).** Measured 2026-08-21: the core branch
  hit 41 via #1102 (42 -> 41, merged), while the descendant chain's own
  #1090 router also decrements 42 -> 41 on its branch (then -> 39 settlement/
  renewal -> 37 backing -> 34 guards/release -> 33 partial-redemption,
  pre-rebase). Two different additions both landing on 41 means the two
  decrements compete for the same slot budget when #1090+ rebase over the
  #1102 fold — the combined storage-layout parity must be re-run against
  the rebased whole, not trusted to each PR's own single-increment parity
  test. This is the concrete storage item in §3 step 2 of
  `epic-merge-plan.md` and belongs on the §5 audit checklist.
- **`reservationsByAnchorUtxo` has two write sites and no removal — corrected
  2026-08-21.** This item previously read: "`reservationsByAnchorUtxo` is
  #1094-line only, removed from the merged base by #1102 — reconcile on
  rebase", and explained that the mapping "was deleted from
  `feat/utxo-reservation-core` by the #1102 merge (which moved anchor
  consumption to `spentMainUTXOs`)". Measured against the branches, that is
  wrong on both counts. The mapping is **introduced by #1091**, not #1094, and
  has 0 hits in `BridgeState.sol` on `feat/utxo-reservation-core`, so #1102 -
  which merged into that branch - had nothing to delete. `spentMainUTXOs` is a
  **pre-existing** Bridge registry that reservations write into
  (`Reservation.sol:1454`, `:1510`; documented at `:66`), present on #1088's
  branch with 6 mentions, so it was not introduced by #1102 either. The two are
  different things, not competing designs: `spentMainUTXOs` is the
  honestly-spent-outpoint registry, `reservationsByAnchorUtxo` is the reverse
  index from anchor outpoint to reservation key.
  The genuine item is narrower: **two write sites** (#1091's and #1094's
  stranding write) must be reconciled in the rewrite, because stranding is one
  of only two position-closing paths reachable under variant B. See
  `inventory/pr-map.md` §4 and `milestone-inventory.md` C-1. Cross-referenced
  from §3.1 and §12 L-01.
- **Positive, verified counterpoint**: CI is currently green on both
  stack tips — `contracts-build-and-test` and `contracts-slither` pass on
  tbtc-v2 #1096, and the full Go suite (`client-build-test-publish`,
  `client-integration-test`, `client-lint`, `client-vet`) passes on
  keep-core #4238 — checked live 2026-08-19, not just self-reported PR-body
  numbers.
- **Review-confirmed decision-relevant risks (2026-08-21 multi-agent
  review; evidence-triggers, NOT decisions).** These design-level findings
  survived adversarial refutation — record them as open team questions, not
  as resolved:
  - **Grinding expropriates the redeeming owner, not the pool.** On the
    redemption path the contract burns the owner's escrowed `mintedAmount`
    while validating the Bitcoin output against `anchorAmount` only
    (`pr-1102 Reservation.sol:740-755`); the header (:53-60) states supply
    and backing reconcile exactly on redemption, so backing-ratio monitoring
    cannot see the owner-side loss; the dissolution path reconciles nowhere
    (:1155-1167). Re-frame followups item 7 with a per-path victim table.
  - **SPV-maintainer stall -> slashing -> termination -> correlated stranding.**
    Every reservation proof is `onlySpvMaintainer`; a dissolution timeout
    slashes the wallet and pays the notifier; two 48h cycles terminate an
    HONEST wallet holding anchors. Record the composition (the elements
    alone are known), enumerate the mainnet `isSpvMaintainer` set, and check
    whether `MaintainerProxy` wires a `submitReservationProof` (none found
    at #1091 — possible no-mainnet-submitter).
  - **§6 financing vs §15 constant-residual contradiction.** §6 describes
    in-kind fees as financed (supply shrinks in lockstep); §15 presupposes
    they are not. Reachable code sides with §15 (re-anchor does not change
    `mintedAmount`, `pr-1102 :990-992`). Resolve against #1093; the
    decoupled `anchorAmount`/`mintedAmount` primitive Space C needs is the
    same one that would close the grinding question.
  - **Space C's `c` is not enforced.** The re-anchor caller chooses the
    target (`ReservationRouter` permissionless), the amount caps are
    `0 disables`, and no cap is relational to any LTV — nothing binds `c`
    at any reachable depth (`shortfall-design-space.md` §7 leaves it open).
  - **Route-1 assessment base can walk before the trigger.** In-kind
    redemption stays open to all healthy holders until `Terminated`;
    under Space C exiting is dominant, so the §4.3 base can be empty at
    attach time. Attach the lien at acceptance or arm on the first liveness
    signal, or answer §7's "is the assessment wanted at all?" in the
    negative (`shortfall-design-space.md`).
  - **The reopen trigger is unevaluable as written**
    (`exit/README.md`). It compares expected annual loss to a carrying cost
    that is nowhere quantified. Estimate termination hazard per wallet-year
    from Bridge event history (a computation finishable now, no new build)
    rather than relying on Tier 0's near-zero realized-stranding sample.
- All 9 PRs started as **DRAFT / REVIEW_REQUIRED, none merged**; that is
  now down to **8 of 9 remaining draft** — `#1102` merged 2026-08-21 into
  the stack root's own branch (`feat/utxo-reservation-core`), so its 30
  fixes sit on #1088's tip rather than as a separate live PR. The
  stack order is meaningful (each depends on its parent's storage/state
  invariants) and should land in the git-stack order given in §Sources.
- **FROST/Schnorr migration compatibility** (forward-looking, non-blocking today — no
  FROST wallet exists yet): re-anchoring a reservation to a future FROST wallet already
  works on the request side; the settlement-side output check in `ReservationProofs.sol`
  calls `extractPubKeyHash`, which the FROST branches deliberately leave P2TR-rejecting.
  A 4-call-site swap to the sibling `extractWalletPubKeyHash` closes this once FROST
  activates. Full analysis: §17 / `frost-reservations-interaction.md`.

---

## 16. Completeness assessment (gap analysis, 2026-08-19; state refreshed 2026-08-21)

**Verdict: not code-complete.** The design is thorough and the Solidity
side is heavily implemented and tested, but three concrete blockers stand
between this and a shippable feature:

1. **The tbtc-v2 stack hasn't merged.** 8 draft PRs, meaningful stack
   order, branches already drifting behind their bases (BEHIND merge
   state on #1094/#1096; **#1090 now CONFLICTING** as of 2026-08-21 after
   `#1102` — itself the one merged PR, into the stack root's branch —
   landed in its base). The stack still lands as one unit per the
   deploy-inert-then-activate constraint (§11).
2. **No external audit gate passed.** The runbook's own pre-audit
   checklist (§11) is unchecked end to end. What's closed so far are
   internal adversarial review-round findings (§12), not a third-party
   audit sign-off — and the findings doc the runbook cites as the source
   of truth for those doesn't exist in the tree.
3. **keep-core cannot execute the shipped design.** `#4238` implements the
   wallet-side shapes for the *original* single-phase model (#1088's
   shape), not the two-phase/renewal/backing/partial-redemption redesign
   that's actually landing in #1091-#1096. Ethereum bindings are stubbed
   with errors, there is no protobuf marshaling (JSON placeholder only),
   no coordination-executor wiring, and no integration tests. A **second,
   currently-unwritten keep-core PR** (still not open as of 2026-08-21 and
   gated on the tbtc-v2 ABI) is required before any wallet node
   can act as custodian under this design — this isn't a follow-up detail,
   it's the entire client-side implementation of the two-phase protocol.

**What is solid**, for balance: the two-phase state machine, snapshot-at-
request discipline, storage-layout append-only policy, and claim-equals-
anchor backing invariant are all implemented (not just designed) and
covered by thousands of lines of Solidity tests that currently pass in CI.
The deployment runbook is detailed and its scripts (`95_deploy_reservation_vault.ts`,
`96_deploy_maintainer_proxy_v2.ts`) exist on-branch, not just as prose.

**Residual gaps even after the above three blockers clear:**
- Two governance parameters (`reservationTxMaxFee`, `feeReserveTarget`)
  have no proposed value yet, let alone sign-off.
- No stranding-compensation mechanism beyond an event stub.
- Reserved-fraction cap enforcement is permanently off-chain by design
  (accepted limitation, not a bug, but worth remembering when reasoning
  about worst-case exposure).
- Re-anchor fee-grinding is capped (#1088's `maxCumulativeReanchorFee`) but
  the cap is unbounded as a fraction of claim value — backing left after
  maximal grinding is a constant (`reservationTxMaxFee + 1 -
  reservationDissolutionTxMaxFee`) regardless of claim size; the bound
  decision is deferred to the #1093 backing review (see §15 / followups
  item 7).
- Three residuals from the #1088/#1102 follow-up reviews remain open-by-
  assumption and are consolidated in §15, each with its verify-action and
  commit pins in `pr-review-followups.md`:
  stranding reachability from all three termination paths (item 1);
  re-anchor/dissolution proof-submission gating after the split (item 3);
  vault-rotation liveness vs a single blocking owner (item 2).
- FROST/Schnorr forward-compatibility patch (§17) is not yet applied — harmless today
  (no FROST wallet exists to re-anchor to), but should land before any reservation is
  ever re-anchored toward a FROST-signed wallet.
- One governance-operational invariant (`vetoDelay < redemptionTimeout`) is
  a procedural reminder in docs, not contract-enforced. The vault-re-pointing
  invariant **is** contract-enforced (§15, `Reservation.sol:1267-1274`).

---

## 17. FROST/Schnorr migration interaction (forward-looking, non-blocking)

A separate, unrelated migration (`tlabs-xyz/frost-upgrade`; tbtc-v2 FROST PR chain
#971->#1027/#972/#973, keep-core #3866->#4199->#4226 and #4005->#4198->#4227) replaces
tBTC v2's threshold-ECDSA (GG20) signer with FROST threshold Schnorr for new P2TR
wallets, coexisting with ECDSA wallets on no committed drain calendar
(`frost-upgrade/docs/adr/0017-fund-migration-own-timeline.md`, ratified 2026-08-10).
Full analysis: `frost-reservations-interaction.md`. Summary relevant to this
spec:

- **No forced sequencing.** Reservations do not need to launch after, or dissolve
  ahead of, FROST. A reservation anchored to a healthy ECDSA wallet operates normally
  through FROST's entire activation program; only an *unhealthy* wallet's own
  365-day `MovingFunds` clock is time-sensitive (§5's stranding bound already covers
  this operationally).
- **The re-anchor mechanism (§4.3) is the intended migration path**, already built
  for exactly this: moving a reservation off a retiring wallet without dissolving it.
  The target-liveness check already recognizes FROST wallets (verified directly
  against the FROST branches, correcting an initial subagent misread that claimed
  otherwise): FROST wallets register into the same `registeredWallets` mapping via a
  synthetic 20-byte compatibility public-key hash.
- **One small, identified settlement-side patch is needed before a re-anchor to a
  FROST wallet can ever settle**: `ReservationProofs.sol`'s 4 output-verification call
  sites use `self.extractPubKeyHash(output)`, which the FROST branches deliberately
  keep P2TR-rejecting; swap to the sibling `extractWalletPubKeyHash` (same signature,
  same return type). Not a design change, not urgent — no FROST wallet exists to
  re-anchor to yet — but should land before FROST activates, tracked in §15/§16.
- **keep-core's future FROST-aware reservation transaction assembly** is new scope
  layered onto the already-identified "second keep-core PR" for the two-phase
  protocol (§13/§16) — not separate follow-up work.
- **Storage merge risk if both stacks are combined**: `BridgeState.sol`, `Bridge.sol`,
  `Wallets.sol` are modified by both PR stacks independently; each stack's own
  storage-layout parity test only covers its own append, not the combined result.
  A combined parity test is needed if/when both land together — not currently
  required, since neither stack depends on the other to ship.
- **Balance accounting was already checked and found correct**: `Wallets.sol`'s
  `getWalletBtcBalance` derives strictly from the wallet's main-UTXO value, never
  `walletReservationsAmount` — reservation-anchored BTC is already excluded from
  FROST's (or any) generic wallet-balance drain arithmetic. No fix needed.
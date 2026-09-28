# UTXO Reservations — Milestone 1 Requirements

Draft: 2026-09-28. Grounded in the M1 code — tbtc-v2 `reservations-upgrade` @
`9f8f5ef1` and keep-core `reservations-epic` @ `f66f11240` — which is
implemented on integration branches, **not merged to `dev`/`main`, not
audited, not deployed** (tracker PRs: tbtc-v2 #1116, keep-core #4282, both
open drafts). Every claim below is subordinate to the dated decisions
recorded in `README.md` and to the scope decision in `roadmap.md` §1: this
document assigns numbered FR/NFR/US identifiers to that scope, it does not
re-litigate it. Where this document and an older file in this folder
disagree on a current-state fact, this document and `README.md` win; where
either disagrees with the code, the code wins.

Companion: `architecture.md` (components, trust boundaries, flows, state
machines, storage, deployment — this document references it by section
number throughout).

---

## 1. Problem statement

tBTC's pooled redemption model sweeps every deposit into a wallet's single
main UTXO; a depositor's specific coin is gone from the moment it is swept,
replaced by a claim on the shared pool. UTXO reservations add a **segregated
custody lane**: a depositor's UTXO is held by the wallet under its own
outpoint (the "anchor"), never commingled with the pool, so that a future
in-kind redemption can return that exact Bitcoin. Milestone 1 builds the
custody rails for that lane — creation (deposit, acceptance, mint) and the
one mechanism that keeps a position alive as wallets rotate (re-anchor) —
without building the in-kind exit itself. `feature-spec.md` §1 has the full
background, including the security-review finding (a single-phase
request/prove model cannot safely authorize a long-lived Bitcoin UTXO
lifecycle) that produced the two-phase, nonce-bound settlement machine every
M1 action uses (`architecture.md` §5).

## 2. Goals

- **G-1.** Let a depositor obtain a segregated, individually traceable
  custody position ("reservation") for a specific UTXO, distinguishable
  on-chain from the pooled deposit path from the moment of reveal
  (`Deposit.sol` `isReserved`, FR-1).
- **G-2.** Mint tBTC against the reservation's anchor 1:1 (minus the
  vault's initiation fee) with no delay beyond the two-phase
  authorize-then-prove settlement window, and keep the tracked claim
  (`mintedAmount`) equal to the current anchor value for the life of the
  position (invariant #1, `feature-spec.md` §14; FR-6, FR-10).
- **G-3.** Keep a reservation alive across routine wallet rotation: when a
  custodying wallet enters `MovingFunds` or `Closing`, or when governance
  chooses to move a healthy position, the anchor must be re-anchorable to
  another `Live` wallet at any position age, with no reachability cliff
  (FR-8, FR-9; measurable: zero on-chain paths gate re-anchor on
  `dissolutionEligibleAt` in M1 code — verified against
  `Reservation.sol:722-858`).
- **G-4.** Bound the blast radius of an unfinished feature: a global
  active-position cap and per-wallet caps must make saturation a revert,
  not a silent failure, and must be checked and reserved at request time,
  never only at proof time (FR-19, FR-20; measurable:
  `maxActiveReservations` is a required-positive launch gate,
  `Reservation.sol:1220-1232`, `:1383-1385`).
- **G-5.** Guarantee that a reserved deposit and its anchor are never
  swept into the pooled main UTXO at any point in their life
  (`DepositSweep.sol:432-445`, invariant #3).
- **G-6.** Keep the storage layout complete for the M2 feature set — every
  field a future redemption/dissolution/renewal design will read
  (`expiresAt`, `dissolutionEligibleAt`, the `ActionType`/`ActionState`
  enum values) must already be written by M1, even where nothing in M1
  reads it back (FR-11, `roadmap.md` §2.1, §0.7 item 3).
- **G-7.** Ship the reservation surface without pushing `Bridge` over the
  EIP-170 deployed-bytecode limit, using a mechanism (delegatecall router)
  that costs the ceremony M2 needs anyway rather than adding a new one
  (NFR-GAS-1, NFR-UPG-1).

## 3. Non-goals

M1 is variant B: an essentials-only rewrite that ships creation, custody
and re-anchor, and cuts everything else. Each cut below is a genuine
absence in the M1 Bridge code, not a deployed-but-paused surface. The
Bridge-side cuts (no dissolution, no redemption, no renewal) trace to the
2026-08-21 variant B scope decision (README Decision 2) and the
2026-09-07 `#1122` revert of `#1121` that removed the reserved-redemption/
veto/renewal surface (README Decision 3). The vault-side cuts (no
upgradeability, no unpause path to M2 features) trace to the Option B
decision of 2026-09-24, confirmed by the project owner on 2026-09-28
(README Decision 4, `m1-b-implementation.md` §3), which superseded the
earlier plan to ship a full vault entry-point surface behind pause flags.

- **No in-kind redemption, whole or partial.** `requestReservedRedemption`
  and the partial-redemption split are M2. `ReservationVault` has no
  `redeemReservation`/`retryRedeemReservation` function in M1 at all
  (verified: 7 external functions total, `ReservationVault.sol:161-349`,
  none of them redemption). Deferred to M2's new-vault-deployment +
  depositor migration ceremony (Decision 4, `README.md`).
- **No renewal.** `extendReservation` is absent from the router;
  `ReservationVault` has no `extendCustody`. `reservationRenewalWindowSeconds`
  is still validated and stored (`Reservation.sol:1286-1289`) for M2's
  benefit, but nothing in M1 reads it. Deferred to M2.
- **No dissolution.** `requestReservationDissolution` does not exist on
  the router; `ActionType.Dissolution` is a declared-but-never-constructed
  enum value kept only for M2 storage/layout stability
  (`Reservation.sol:109`). `dissolutionEligibleAt` is still written at
  every term grant (G-6) but has exactly one on-chain reader in M1: the
  stranding precondition for a `Closing` wallet (FR-9). Deferred to M2.
- **No partial redemption.** Subsumed by the redemption non-goal above;
  `isPartial` is a declared struct field, unused in M1.
- **No emergency exit.** The `exit/` folder's standing-committee design
  (Mechanism 1) is deferred design reference, not scoped or funded
  (Decision 1, `README.md`; `exit/README.md`). `Stranded` is M1's only
  terminal fallback for a dead wallet (FR-9).
- **No vault upgradeability.** `ReservationVault` is a plain `Ownable`
  contract with immutable constructor arguments, not a proxy
  (`ReservationVault.sol:46-136`). Re-pointing `Bridge.reservationVault`
  to a new vault requires total quiescence
  (`reservationTotalAmount == 0 && pendingReservedDeposits == 0`,
  `Reservation.sol:1307-1314`) — under B that state is reachable only by
  terminating every custodying wallet (`docs/RESERVATION_CAPS_DEPLOYMENT.md`
  "Irreversible Vault Activation Warning"). M2's redemption/renewal ship via
  a **new vault deployment and a depositor opt-in migration ceremony**, not
  an unpause flag on this vault (Decision 4, confirmed by the project owner
  on 2026-09-28).
- **No watchtower veto.** `notifyReservedRedemptionVeto` is absent; the
  `watchtowerDefaultDelay`/`LevelOneDelay`/`LevelTwoDelay` action fields are
  declared but only ever written by the M2 redemption request path. Vacuous
  in M1 because no redemption exists to veto.
- **No wallet allowlist for reservation-eligible wallets.** Depositors pick
  any `Live` wallet as the designated wallet at reveal; roadmap.md §7 item
  3 records a 2026-09-07 decision to add one, but grepping the M1 code
  (`ReservationRouter.sol`, `Reservation.sol`) finds no allowlist mapping or
  setter — **not implemented**, open per §12.
- **No structural bound on cumulative re-anchor fee loss as a fraction of
  the claim** (only the flat dust floor `anchorAmount > txMaxFee +
  minAmount` per hop, `Reservation.sol:805-808`); `cumulativeReanchorFee`
  is tracked (`ReservationProofs.sol:797`) but not capped. Open per §12.

## 4. Scope: M1 vs M2

| Capability | M1 | M2 |
|---|---|---|
| Deposit reveal to reservation vault, segregation | Built | — |
| Acceptance (two-phase: request, SPV proof) | Built | — |
| Mint against anchor, initiation fee | Built | — |
| Re-anchor (governance-driven and retiring-wallet permissionless) | Built, unbounded in position age | Restoring an eligibility gate is an open inherited decision (`roadmap.md` §7 item 4) |
| Action timeout / unwind | Built (Acceptance and Reanchor only) | Extends to Redemption/Dissolution |
| Stale reserved-deposit cleanup (permissionless + governance force) | Built | — |
| Stranding (terminal fallback for a dead/closed wallet) | Built | — |
| Caps: per-wallet count/amount, global active-position count, single-reservation amount | Built, request-time reserved | Values re-tuned as needed |
| Governance parameter/cap updates with relational validation | Built | — |
| In-kind fee financing, debt, sweep | Built (financing applies from the first re-anchor onward) | — |
| In-kind whole/partial redemption | Absent | Built, new vault + migration |
| Renewal | Absent | Built |
| M2 delivery mechanism | — | New `ReservationVault` deployment + depositor migration (confirmed by the project owner 2026-09-28; §12) |
| Dissolution | Absent | Built, restores or redesigns re-anchor's eligibility gate |
| Watchtower veto | Absent | Built |
| Reservation-eligible wallet allowlist | Absent (open decision) | Candidate for M1 backport or M2 |
| Retry credit | Declared field only | Built |
| Standing-committee emergency exit | Not built, design reference only | Reopens only on stranding-frequency/market evidence (`exit/README.md`) |

## 5. Actors

| Actor | Capabilities in M1 | Trust assumption |
|---|---|---|
| Depositor / reservation owner | Sends BTC to a tBTC wallet, reveals with the reservation vault named, requests acceptance for their own deposit, receives tBTC minted against the anchor | Untrusted; every check that matters is on-chain. Cannot move, redeem, or close the position in M1 |
| Custodian wallet & its signers (keep-core) | Signs the anchor and re-anchor Bitcoin transactions once a Bridge authorization exists; runs the acceptance/re-anchor tasks, the SPV proof submitters, and the action-timeout/stranding/stale-deposit watchers | Threshold-honest-majority trust identical to pooled tBTC custody (51-of-100 mainnet); fraud challenges and slashing apply to any signature outside a pending authorization |
| `ReservationVault` (contract) | Mints/distributes tBTC on Bridge credit, finances in-kind re-anchor fees, records/repays fee debt, sweeps excess reserve | No claim registry of its own; trusts the Bridge's reservation records for ownership. `onlyBank`-gated mint entry, `onlyOwner`-gated fee/sweep setters |
| Vault owner (Threshold Council, via `BridgeGovernance`) | Owns the vault; sets `initiationFeeBps` (capped 5%) and `feeReserveTarget` instantly; sweeps fees | Same governance trust as the rest of the Bridge; instant (non-delayed) setters are a deliberate M1 choice pending a possible future delay |
| SPV maintainer | Submits acceptance and re-anchor SPV proofs (`onlySpvMaintainer`) | Approved allow-list (`self.isSpvMaintainer`); cannot forge a proof or change what it settles — Bitcoin PoW/Merkle/coinbase checks are unconditional |
| Permissionless callers / keepers | File every M1 housekeeping notification with no reward: action timeout, stale-deposit release, stranding, fee-debt repayment | Untrusted by design — every one of these calls is a bookkeeping transition the contract itself fully validates; no call can move funds beyond what the state machine already authorized |
| Governance (`BridgeGovernance` on behalf of Bridge) | `updateReservationParameters`, `updateReservationCaps`, `setReservationRouter` (once), `forceStaleReservedDeposit`, privileged re-anchor of a healthy (`Live`-wallet) position | Same staged begin/finalize governance-delay trust as the rest of the Bridge; every reservation setter validates its own relational invariants on-chain (FR-11, FR-12) |
| Wallet registry / DAO (indirect) | Determines wallet lifecycle transitions (`Live`/`MovingFunds`/`Closing`/`Terminated`/`Closed`) that gate re-anchor eligibility and the stranding precondition | Out of this feature's direct control surface; reservations consume wallet state, they do not produce it |

## 6. User stories

Each story cites the enforcing M1 code and, where one exists, the
per-repo story ID in `m1-tbtc-v2-readiness/02-user-stories.md` (tbtc-v2#N)
or `m1-keep-core-readiness/02-user-stories.md` (keep-core#N). Where a
readiness doc's own "current status" note is stale against the M1 code
verified here, that is flagged rather than repeated.

### US-1 — Deposit reveal reserves capacity
As a depositor, I want to reveal my deposit with the reservation vault
named, so that my UTXO is tracked as a reservation candidate instead of
being swept into the pool.
- **Given** a fresh SegWit deposit funding transaction and a chosen
  designated wallet,
- **When** I call `revealDeposit`/`revealDepositWithExtraData` naming the
  reservation vault as `vault`,
- **Then** the Bridge sets `isReserved = true`, records
  `pendingReservedDeposit[key]` with the designated wallet and refund
  deadline, and increments `pendingReservedDeposits`
  (`Deposit.sol:215-238,386-395`); a later `DepositSweep` attempt on this
  UTXO reverts (`DepositSweep.sol:432-445`).
- Maps to tbtc-v2#1 (implemented, landed).

### US-2 — Owner requests acceptance
As the reservation owner, I want to request acceptance for my revealed
deposit, so that the designated wallet is authorized to anchor it and my
capacity is reserved before any Bitcoin transaction is signed.
- **Given** a revealed, non-swept reserved deposit with headroom under the
  single-reservation, per-wallet, and global active-position caps,
- **When** I (the depositor) call `requestReservationAcceptance`,
- **Then** the Bridge reserves capacity, snapshots `txMaxFee`, `minAmount`,
  `termSeconds`, `dissolutionDelay` into a new `Pending` `Acceptance`
  action, and opens a signing window that fits before the deposit's refund
  deadline (`Reservation.sol:487-638`).
- **Given** the caller is not the deposit's depositor, or the amount is
  below `reservationMinAmount + reservationTxMaxFee`, or any cap would be
  exceeded,
- **When** `requestReservationAcceptance` is called,
- **Then** it reverts with the specific `require` message (15 distinct
  checks, `Reservation.sol:492-602`).
- Maps to tbtc-v2#2 (implemented) and keep-core#1 (`pkg/tbtcpg/reservation_acceptance.go:56` `Run`).

### US-3 — Custodian wallet proves acceptance
As the custodian wallet's signing group, I want to sign and prove the
anchor transaction, so that the reservation settles to `Active` and the
owner is minted tBTC.
- **Given** a `Pending` acceptance authorization not yet timed out,
- **When** the wallet signs a 1-in-1-out anchor transaction and an SPV
  maintainer calls `submitReservationAcceptanceProof`,
- **Then** the Bridge writes `owner`, `mintedAmount`, `acceptedAt`,
  `walletPubKeyHash`, `anchorAmount`, `expiresAt`, `anchorTxHash`,
  `state = Active`, and `dissolutionEligibleAt = expiresAt +
  dissolutionDelay`, then credits the vault, which mints tBTC and pays the
  owner minus the initiation fee (`ReservationProofs.sol:473-626`;
  `ReservationVault.sol:161-197`).
- Maps to tbtc-v2#3 (implemented; unit-covered
  `Bridge.ReservationAcceptanceAuthorization.test.ts`,
  `Bridge.ReservationSettlement.test.ts`).

### US-4 — Acceptance authorization times out
As anyone, I want to report an expired, unsigned acceptance authorization,
so that its reserved capacity is released for a fresh attempt.
- **Given** a `Pending` acceptance action whose `timeoutAt` has elapsed,
- **When** any caller calls `notifyReservationAcceptanceTimedOut`,
- **Then** the action is marked `TimedOut`, its reserved capacity
  (wallet count/amount, global total, active-position count) is released,
  and the deposit stays available for a later acceptance request; a late
  proof against the timed-out generation still settles it
  (`Reservation.sol:655-691`; `ReservationProofs.sol:493-539` late path).
  No slashing occurs on this path — it is a capacity release, not a wallet
  liveness failure.
- Maps to tbtc-v2#4 (implemented) and keep-core#2. Note: keep-core#2's own
  "Caveat" (watcher registration had zero production callers) is stale
  against the current M1 code — `WireReservationWatchers`
  (`pkg/maintainer/spv/reservation_wiring.go:134-536`) now constructs and
  starts all three reservation watchers with real wallet discovery; no
  "PR H placeholder" comment remains in that file.

### US-5 — Governance re-anchors a healthy reservation
As governance, I want to move a reservation off a healthy `Live` wallet, so
that I can rebalance custody without waiting for the wallet to retire.
- **Given** an `Active` reservation on a `Live` source wallet, a different
  `Live` target wallet with headroom, and no cooldown in effect,
- **When** `BridgeGovernance` (i.e. `msg.sender == governance`) calls
  `requestReservationReanchor`,
- **Then** the request succeeds **at any position age** — M1 code has no
  `dissolutionEligibleAt` gate on re-anchor at all
  (`Reservation.sol:722-858`, confirmed by full read: no reference to
  `dissolutionEligibleAt` in the function body).
- Maps to tbtc-v2#5. **Correction:** that readiness doc's "current status"
  records a confirmed deviation (a `dissolutionEligibleAt` gate still
  present and unconditional) against an earlier snapshot (`landed tip
  8c5a2f4d`); re-verified against `9f8f5ef1`, the gate does not exist. The
  deviation was fixed between those two refs. Restoring some form of this
  gate at M2 dissolution design time is the still-open item in §12.

### US-6 — Retiring wallet's reservation re-anchors away
As anyone, I want a `MovingFunds`/`Closing` wallet's reservations moved to
a `Live` wallet, so that the retiring wallet can eventually close without
stranding the position.
- **Given** an `Active` reservation on a source wallet in `MovingFunds` or
  `Closing`, a different `Live` target with headroom, and
  `anchorAmount > reservationTxMaxFee + reservationMinAmount`,
- **When** anyone calls `requestReservationReanchor` (no `privileged`
  requirement for a retiring source),
- **Then** capacity is reserved on the target, the action authorizes a
  1-in-1-out transaction, and on proof the anchor migrates, the claim
  shrinks by the miner fee, and the vault finances that fee
  (`Reservation.sol:722-858`; `ReservationProofs.sol:640-761`).
- Maps to tbtc-v2#6 and keep-core#6. **Automated only for a
  `MovingFunds` source:** `ReservationReanchorTask.Run` gates on
  `walletChainData.State == StateMovingFunds`
  (`pkg/tbtcpg/reservation_reanchor.go:65-83,139`); target discovery in
  `findTargetWallet`/`scanForTargetWallet`,
  `pkg/tbtcpg/reservation_reanchor.go:439-599`. **Not automated for a
  `Closing` source** — the contract accepts it (FR-5, FR-6), but no
  keep-core task triggers on `Closing`; draining it depends on a
  permissionless `requestReservationReanchor` caller, a gap tracked in
  `m1-keep-core-readiness/01-gap-analysis.md` (Major row) and open per
  §12.

### US-7 — Re-anchor authorization times out
As anyone, I want an unproven re-anchor authorization released after its
deadline, so that the reservation returns to normal and can be retried.
- **Given** a `Pending` `Reanchor` action past `timeoutAt`,
- **When** any caller calls `notifyReservationActionTimeout`,
- **Then** the target wallet's reserved capacity is released, the
  reservation returns to `Active` on its original (source) wallet, and a
  `reanchorCooldownUntil` is set equal to one more timeout-length window
  before a non-privileged caller may request again
  (`Reservation.sol:860-914`). No slashing on this path either.
- Maps to tbtc-v2 (part of #6/#7's flow, no dedicated readiness-doc story
  number) and keep-core#2's sibling watcher
  (`pkg/maintainer/spv/reservation_action_timeout_watch.go`).

### US-8 — Custodian wallet proves a re-anchor
As the target wallet's signing group, I want to prove a signed re-anchor
transaction, so that custody formally transfers and the claim is
rebalanced.
- **Given** a `Pending` `Reanchor` action and an unspent source anchor,
- **When** an SPV maintainer calls `submitReservationReanchorProof`,
- **Then** the Bridge releases the source wallet's count/amount, migrates
  the reverse anchor index, rewrites `walletPubKeyHash`/`anchorAmount`/
  `mintedAmount`/`anchorTxHash` to the new hop, accumulates
  `cumulativeReanchorFee`, and (if a fee was charged) calls
  `financeInKindFee` on the vault **last**, after every other effect has
  committed (`ReservationProofs.sol:640-809`, checks-effects-interactions
  ordering documented at `:753-757`).
- Maps to tbtc-v2#7 (implemented; unit-covered
  `Bridge.ReservationSettlement.test.ts`,
  `Bridge.ReservationSourceAnchorBinding.test.ts`) and keep-core (SPV
  submitter `pkg/maintainer/spv/reservation_reanchor_proof.go`).

### US-9 — Wallet closes only after shedding every reservation
As the protocol, I want a wallet's voluntary close to be blocked while it
still custodies a reservation, so that an anchor is never abandoned by a
wallet lifecycle transition.
- **Given** a wallet in `Closing` whose closing period has elapsed,
- **When** `notifyWalletClosingPeriodElapsed` is called,
- **Then** it reverts with `"Wallet still custodies reservations"` unless
  `walletReservationInfo[wallet].count == 0` (`Wallets.sol:360-386`).
- Maps to tbtc-v2#8 (implemented; unit-covered).

### US-10 — Reservation on a dead wallet is stranded
As anyone, I want a reservation on a terminated/closed (or eligibility-
lapsed closing) wallet marked `Stranded`, so that its capacity is released
even though the wallet will never cooperate again.
- **Given** an `Active` reservation whose custodying wallet is
  `Terminated`, `Closed`, or `Closing` with `block.timestamp >=
  dissolutionEligibleAt`,
- **When** anyone calls `notifyReservationStranded`,
- **Then** capacity is released, the reverse anchor index entry is deleted,
  the reservation becomes `Stranded`, `ReservationStranded` fires, and the
  owner's tBTC balance is unaffected (already minted at acceptance) —
  they become an ordinary pooled-tBTC holder for that value
  (`Reservation.sol:985-1115`; `exit/stranded.md` §1-§2).
- Maps to tbtc-v2#9 and keep-core#3/#4/#5 for the `Terminated`/`Closed`
  cases (cause-agnostic watcher,
  `pkg/maintainer/spv/reservation_stranding_watch.go:84-204`, triggered
  by `OnWalletClosed`/the startup scan/`drainStrandingRechecks`, all of
  which gate on `StateClosed`/`StateTerminated`). **No client trigger
  exists for the third on-chain-eligible case** — a `Closing` wallet
  reaching `dissolutionEligibleAt` — so stranding it depends on a
  permissionless caller filing `notifyReservationStranded` manually, a
  gap tracked in `m1-keep-core-readiness/01-gap-analysis.md` (Major row,
  "Stranding coverage gap for `Closing`-past-`dissolutionEligibleAt`
  wallets") and open per §12.

### US-11 — Stale reserved deposit released permissionlessly
As anyone, I want an unaccepted reserved deposit released once its refund
deadline passes, so that its slot in `pendingReservedDeposits` stops
blocking a governance vault change and the depositor's normal Bitcoin
refund path is unblocked.
- **Given** a pending reserved deposit with no pending acceptance
  authorization, past its snapshotted refund deadline,
- **When** anyone calls `notifyStaleReservedDeposit`,
- **Then** the pending-reserved-deposit entry is cleared and
  `pendingReservedDeposits` decrements (`Reservation.sol:1117-1165`).
- Maps to keep-core's stale-deposit watcher
  (`pkg/maintainer/spv/reservation_stale_deposit_watch.go`).

### US-12 — Governance force-clears a griefing stale deposit
As governance, I want to force-clear a pending reserved deposit before its
own refund deadline, so that a depositor who reveals and never funds the
anchor cannot indefinitely grind the pending-deposit guard against honest
depositors.
- **Given** a pending reserved deposit with no pending acceptance
  authorization (refund deadline irrelevant),
- **When** governance calls `forceStaleReservedDeposit`,
- **Then** the same clearing effect as US-11 applies immediately
  (`Reservation.sol:1167-1213`).

### US-13 — Coordinator respects caps before proposing
As the keep-core coordinator generating an acceptance proposal, I want to
reject a candidate deposit before broadcasting it, so that a proposal
never wastes a signing round on a request the Bridge will revert.
- **Given** live `ReservationParameters`/caps fetched fresh for the current
  proposal,
- **When** a candidate would exceed `MaxReservationsPerWallet`, fall below
  `ReservationMinAmount`, or push `ReservationTotalAmount` past
  `ReservationMaxTotalAmount`,
- **Then** the coordinator excludes it before proposing
  (`pkg/tbtcpg/reservation_acceptance.go:567-580`, calling
  `checkReservationAcceptanceEligibility`, `:977-1078`).
- Maps to keep-core#7 and #9 (live parameter refresh,
  `pkg/tbtcpg/reservation_acceptance.go:339`, inside
  `findReservationAcceptanceCandidate`).

### US-14 — Governance updates reservation parameters and caps
As governance, I want to change reservation parameters/caps through the
staged delay process, so that every update passes the on-chain relational
checks before it takes effect.
- **Given** a staged `updateReservationParameters` or `updateReservationCaps`
  call past its governance delay,
- **When** `BridgeGovernance` finalizes it,
- **Then** the Bridge validates every per-field require (9 for parameters,
  1 plus the shared relational check for caps) and the Decision-1
  relational invariant `reservationMaxTotalAmount <= maxActiveReservations
  * reservationMaxSingleAmount` (skipped when either factor is zero)
  before writing storage (`Reservation.sol:1220-1406`).
- Maps to tbtc-v2#10.

### US-15 — Vault owner and the public manage fee economics
As the vault owner, I want to size the fee reserve and sweep the excess,
and as anyone, I want to be able to repay outstanding in-kind fee debt, so
that re-anchor's Bitcoin miner fees are financed without ever blocking
settlement.
- **Given** a re-anchor settling with a non-zero miner fee,
- **When** `financeInKindFee` is called by the Bridge,
- **Then** it burns what the reserve can cover and records any shortfall
  as public `inKindFeeDebtSat` — **the call never reverts for insufficient
  reserve** (`ReservationVault.sol:199-234`).
- **Given** outstanding debt,
- **When** anyone calls `repayInKindFeeDebt`,
- **Then** TBTC is pulled from the caller and burned, reducing the debt
  (`ReservationVault.sol:236-266`).
- **Given** the vault owner wants to move excess reserve to treasury,
- **When** `sweepFees` is called,
- **Then** outstanding debt is repaid first from the current balance, then
  the excess over `feeReserveTarget` is transferred
  (`ReservationVault.sol:286-319`).

### US-16 — Signer software validates a proposal before broadcasting
As a wallet signer, I want to independently validate an acceptance or
re-anchor proposal against live chain state before signing, so that I
never sign an unauthorized or stale reservation transaction.
- **Given** a generated `ReservationAnchorProposal` or
  `ReservationReanchorProposal`,
- **When** the signer calls `ValidateReservationAnchorProposal` /
  `ValidateReservationReanchorProposal`,
- **Then** the call reaches the real `WalletProposalValidator` contract
  functions (`WalletProposalValidator.sol:966,1143`) via
  `pkg/chain/ethereum/tbtc.go:482-647`, and an invalid proposal is
  hard-rejected before any signature is produced.
- Maps to keep-core#8.

### US-17 — Operators monitor occupancy, timeouts, stranding, and fee debt
As a node operator, I want dashboards and alerts for the M1 saturation and
liveness risks, so that I act before a cap-exhaustion or wallet-retirement
edge case becomes an incident.
- **Given** the reservation subsystem is active,
- **When** occupancy, per-wallet counts, pending action timeouts, or
  in-kind fee debt change,
- **Then** the operator's tooling surfaces it: `ReservationOccupancyChanged`
  events and the four saturation gauges
  (`active_reservations_count`, `max_active_reservations`,
  `live_wallets_count`, `wallet_reservations_count`,
  `pkg/clientinfo/performance.go:738-741`, registered per
  `pkg/clientinfo/performance_test.go:717-757`) for occupancy; the
  action-timeout watcher's poll loop for pending deadlines; the stranding
  watcher for dead-wallet cleanup. **Gap:** no chain-read or gauge exists
  today for `inKindFeeDebtSat` or the fee reserve balance anywhere in
  `pkg/` (confirmed: zero matches for `InKindFeeDebt`/`FeeReserve` in the
  keep-core tree) — open per §12.

### US-18 — Anyone verifies a reservation's state independently
As any observer (an indexer, an auditor, a depositor checking their own
position), I want to read a reservation's full state and lineage from
public view functions, so that I don't have to trust an off-chain summary.
- **Given** a `reservationKey`,
- **When** I call `reservations(key)`, `reservationActions(key, nonce)`,
  `reservationByAnchorUtxo(txHash, index)`, `walletReservationsAmount`,
  `walletReservationsCount`, `activeReservationsCount`, or
  `reservationParameters`,
- **Then** I get the exact on-chain struct/values with no additional
  trust assumption (`ReservationRouter.sol:449-597`, 11 view functions).

## 7. Functional requirements

Each entry is a testable `MUST`/`MUST NOT`. "Enforcing code" cites the M1
snapshot the requirement was verified against; "test" cites a real path in
that snapshot or states `no test found`.

| ID | Requirement | Rationale | Enforcing code | Test |
|---|---|---|---|---|
| FR-1 | The system MUST mark a deposit revealed with the reservation vault as `isReserved` and MUST NOT allow a reserved deposit into `DepositSweep`. | Segregated custody is the product's defining property (G-5). | `Deposit.sol:215-238`; `DepositSweep.sol:432-445` | `Bridge.Deposit.test.ts:1148-1285` (`isReservedDeposit` context) |
| FR-2 | `requestReservationAcceptance` MUST require the caller to be the deposit's own depositor, the deposit to be revealed/reserved/unswept/routed to the vault, the wallet to be the designated `Live` wallet, and every cap (single, per-wallet count/amount, global active-position) to have headroom, before reserving capacity. | Prevents front-running someone else's deposit and prevents capacity from being reserved for a request that can never settle. | `Reservation.sol:487-638` | `Bridge.ReservationAcceptanceAuthorization.test.ts` |
| FR-3 | `submitReservationAcceptanceProof` MUST write `owner`, `mintedAmount`, `acceptedAt`, `walletPubKeyHash`, `anchorAmount`, `expiresAt`, `anchorTxHash`, `state = Active`, and `dissolutionEligibleAt` together, on both the on-time and late-settlement paths. | Storage-completeness (G-6) and claim≡anchor (invariant #1) both depend on this write being atomic. | `ReservationProofs.sol:473-626` | `Bridge.ReservationSettlement.test.ts` |
| FR-4 | `notifyReservationAcceptanceTimedOut` MUST release the exact capacity reserved at request time and MUST leave the generation settleable by a later proof. | A timeout must free the position for another attempt without destroying the confirmed-Bitcoin-spend guarantee. | `Reservation.sol:655-691`; late path `ReservationProofs.sol:493-539` | `Bridge.ReservationAcceptanceAuthorization.test.ts` |
| FR-5 | `requestReservationReanchor` MUST require `privileged` (i.e. `msg.sender == governance`) when the source wallet is `Live`, and MUST allow any caller when the source is `MovingFunds` or `Closing`; it MUST NOT gate on `dissolutionEligibleAt` at any position age. | G-3: re-anchor is the sole unpin path and must not have a reachability cliff. | `Reservation.sol:722-858` (no `dissolutionEligibleAt` reference in the function) | `Bridge.ReservationSettlement.test.ts` ("Reanchor would fall below the minimum reservation amount" and neighbors) |
| FR-6 | `requestReservationReanchor` MUST reject a request that would leave the anchor at or below `reservationTxMaxFee + reservationMinAmount`. | Bounds cumulative fee-grinding across repeated re-anchor hops (§9). | `Reservation.sol:797-808` | `Bridge.ReservationSettlement.test.ts` |
| FR-7 | `notifyReservationActionTimeout` MUST restore the reservation to `Active` on its original wallet and MUST set a `reanchorCooldownUntil` equal to one more timeout-window before a non-privileged caller may re-request. | Bounds request-spam against a target wallet's capacity (`docs/RESERVATION_CAPS_DEPLOYMENT.md` "Permissionless Re-Anchor Target Selection"). | `Reservation.sol:860-914` | `Reservation.test.ts:1609-1612` |
| FR-8 | `submitReservationReanchorProof` MUST call `financeInKindFee` last, after every accounting and stranding-check effect has committed. | Checks-effects-interactions: an untrusted vault reentering during financing must only ever observe a fully settled generation. | `ReservationProofs.sol:640-761` (comment at `:753-757`) | `Bridge.ReservationSourceAnchorBinding.test.ts` |
| FR-9 | `notifyReservationStranded` MUST require the reservation `Active` and the custodying wallet `Terminated`, `Closed`, or `Closing` with `block.timestamp >= dissolutionEligibleAt` — never a `Closing` wallet before that timestamp. | Prevents a permissionless strand from racing a legitimate in-window re-anchor off a `Closing` wallet. | `Reservation.sol:1066-1115` | `Bridge.ReservationStranding.test.ts`, `Bridge.ReservationStrandingLibrary.test.ts` |
| FR-10 | `strandReservation` MUST leave the owner's minted tBTC balance unchanged and MUST NOT mark the anchor as honestly spent. | The loss is the in-kind option, not the tBTC (`exit/stranded.md` §1); the anchor must stay recognizable in the fraud/dispute record. | `Reservation.sol:985-1028` | `Bridge.ReservationStrandingLibrary.test.ts` |
| FR-11 | `notifyStaleReservedDeposit` MUST require no pending acceptance authorization and the snapshotted refund deadline to have elapsed; `forceStaleReservedDeposit` (governance-only) MUST require no pending acceptance authorization but MUST NOT check the refund deadline. | Permissionless release needs the depositor's own refund-path guarantee to have kicked in; governance's griefing-mitigation override does not. | `Reservation.sol:1117-1213` | `notifyStaleReservedDeposit`: `Bridge.ReservationStranding.test.ts`, `Bridge.ReservationStrandingLibrary.test.ts`. `forceStaleReservedDeposit`: no test found (grepped both files and `Reservation.test.ts` — the function appears only in the ABI snapshot fixture, not in any `describe`/`it` block). |
| FR-12 | `updateReservationParameters` MUST validate, before writing any field: `reservationTxMaxFee > 0`; `reservationMinAmount > reservationTxMaxFee`; `reservationTermSeconds` within protocol bounds; `reservationRenewalWindowSeconds` within `(0, reservationTermSeconds)`; `reservationActionTimeout` above the safety margin; `maxReservationsPerWallet > 0`; the Decision-1 cap/slot relation; and, only when actually re-pointing to a new non-zero vault, both the launch gate (`maxActiveReservations > 0`) and quiescence (`reservationTotalAmount == 0 && pendingReservedDeposits == 0`) — nine checks in total, the last two conditional on the vault argument changing. | A partially-invalid parameter set must never reach storage. | `Reservation.sol:1260-1354` | `Bridge.ReservationCaps.test.ts` (governance-parameter section) |
| FR-13 | `updateReservationCaps` MUST require `maxActiveReservations > 0` and MUST validate the Decision-1 relational invariant against the stored `reservationMaxTotalAmount` before writing. | `maxActiveReservations > 0` is the M1 launch gate (no reservation can be accepted before it is set). | `Reservation.sol:1376-1406` | `Bridge.ReservationCaps.test.ts` ("bootstrap ordering" group) |
| FR-14 | `Wallets.notifyWalletClosingPeriodElapsed` MUST revert if `walletReservationInfo[wallet].count != 0`. | A wallet must never finalize closing while custodying an anchor (US-9). | `Wallets.sol:360-386` | `Bridge.Wallets.test.ts` |
| FR-15 | `ReservationVault.receiveBalanceIncrease` MUST mint the full gross amount and distribute it to depositors minus `initiationFeeBps`, retaining the fee in the vault's own balance. | G-2: claim always equals the gross anchored amount before fee. | `ReservationVault.sol:161-197` | `ReservationVault.test.ts` |
| FR-16 | `financeInKindFee` MUST NOT revert for insufficient reserve; any shortfall MUST be recorded as `inKindFeeDebtSat`. | "A confirmed Bitcoin spend must never fail to settle because of the reserve level" (`ReservationVault.sol:209-214`). | `ReservationVault.sol:215-234` | `ReservationVault.test.ts` |
| FR-17 | `updateInitiationFee` MUST reject any value above `MAX_FEE_BASIS_POINTS` (500 = 5%). | Hard ceiling on the one fee depositors cannot avoid. | `ReservationVault.sol:321-339` | `ReservationVault.test.ts` |
| FR-18 | The `ReservationRouter` MUST declare exactly one storage variable (`self`) and MUST NOT shadow any Bridge-declared selector; `Bridge.setReservationRouter` MUST be callable at most once. | Storage-parity and no-standalone-authority are the invariants the delegatecall architecture depends on (G-7). | `ReservationRouter.sol:57-103`; `Bridge.sol:2100-2104`; `BridgeState.sol:1077-1080` | `Bridge.RouterStorageParity.test.ts`, `Bridge.RouterSelectorDisjointness.test.ts`, `Bridge.StorageLayout.test.ts` |
| FR-19 | `Reservation.reserveAcceptanceCapacity` MUST check and increment `activeReservationsCount` against `maxActiveReservations`, the wallet count against `maxReservationsPerWallet`, and (if non-zero) the wallet amount against `maxReservationsAmountPerWallet`, at request time. | Caps are request-time throttles, not proof-time checks (G-4). | `Reservation.sol:916-958` | `Bridge.ReservationOccupancy.test.ts`, `Bridge.ReservationCaps.test.ts` |
| FR-20 | `requestReservationAcceptance`/`requestReservationReanchor` MUST reject a deposit whose amount exceeds `reservationMaxSingleAmount` (when non-zero). | Bounds single-position blast radius independent of occupancy. | `Reservation.sol:598-602` | `Bridge.ReservationCaps.test.ts` |
| FR-21 | `pkg/tbtcpg` `ReservationAcceptanceTask` MUST fetch `ReservationParameters` fresh on every proposal generation, not a cached copy. | A cached-parameters bug would silently apply stale caps/fees after a governance update. | `pkg/tbtcpg/reservation_acceptance.go:339` (inside `findReservationAcceptanceCandidate`, called fresh on every `Run` invocation) | `pkg/tbtcpg/reservation_acceptance_test.go` (`TestReservationAcceptanceTask_ReservationParametersFetchedLive`) |
| FR-22 | `pkg/tbtcpg` `checkReservationAcceptanceEligibility` MUST reject a candidate before proposing if it would exceed `MaxReservationsPerWallet`, fall below `ReservationMinAmount`, or push `ReservationTotalAmount` over `ReservationMaxTotalAmount`. | Avoids wasting a signing round on a proposal the Bridge will revert. | `pkg/tbtcpg/reservation_acceptance.go:977-1078` | `pkg/tbtcpg/reservation_acceptance_test.go` (`TestReservationAcceptanceTask_AmountCapBoundaries`) |
| FR-23 | `ReservationReanchorTask` MUST trigger on a source wallet's `WalletMovingFunds` transition and MUST select a `Live` target wallet with headroom, excluding wallets already targeted by another in-flight re-anchor from the same task instance; it MUST NOT trigger for a `Closing` source. | G-3: re-anchor off a retiring wallet must be automatic, not manual — but this automation stops at `MovingFunds`. A `Closing` source is contract-eligible (FR-5, FR-6) yet has no keep-core trigger; draining it is a permissionless-caller-only gap tracked in `m1-keep-core-readiness/01-gap-analysis.md` (Major row), open per §12. | `pkg/tbtcpg/reservation_reanchor.go:65-83,139,438-599` | `pkg/tbtcpg/reservation_reanchor_test.go` (`TestReservationReanchorTask_TargetWalletExclusion_SharedTask`) |
| FR-24 | `ReservationReanchorTask.notifyMovingFundsBelowDustIfEligible` MUST call the permissionless below-dust report once a retiring wallet's reservation count has reached zero and its remaining balance is below dust. | The only remaining route to close a wallet that proved its funds moved while still holding anchors (`roadmap.md` §0.8). | `pkg/tbtcpg/reservation_reanchor.go:601-737` | `pkg/tbtcpg/reservation_reanchor_test.go` (`TestReservationReanchorTask_Run_NotifiesMovingFundsBelowDust`) |
| FR-25 | `ReservationActionTimeoutWatcher.checkReservationActionTimeout` MUST call `NotifyReservationAcceptanceTimedOut` for an overdue `Acceptance` action and `NotifyReservationActionTimeout` for an overdue `Reanchor` action, and MUST skip (not misdispatch) any other action type. | The Bridge exposes two distinct entry points; calling the wrong one reverts. | `pkg/maintainer/spv/reservation_action_timeout_watch.go:486-646` | `pkg/maintainer/spv/reservation_action_timeout_watch_test.go` |
| FR-26 | `reservationStrandingWatcher.checkReservationStrandingForWallet` MUST notify stranding for every `Active` reservation on a wallet it observes as `Terminated`/`Closed`, identically regardless of which of the three termination causes fired; keep-core has no automated trigger for the third on-chain-eligible precondition. | Cause-agnostic by design for the two states it covers — the reservation-facing consequence is the same regardless of termination cause (`exit/stranded.md` §1). A `Closing` wallet past `dissolutionEligibleAt` has no client trigger and relies solely on a permissionless caller, a gap tracked in `m1-keep-core-readiness/01-gap-analysis.md` (Major row), open per §12. | `pkg/maintainer/spv/reservation_stranding_watch.go:84-204` | `pkg/maintainer/spv/reservation_stranding_watch_test.go` |
| FR-27 | `ReservationStaleDepositWatcher.CheckStaleReservedDeposit` MUST NOT notify for a deposit whose designated wallet has already reached `Live`, and MUST re-arm (park then reconcile) rather than permanently drop such a deposit if the wallet later leaves `Live` without anchoring. | A wallet observed `Live` is expected to anchor the deposit itself; the watcher must not race a legitimate in-flight acceptance. | `pkg/maintainer/spv/reservation_stale_deposit_watch.go:208-488,860-902` | `pkg/maintainer/spv/reservation_stale_deposit_watch_test.go` |
| FR-28 | `pkg/chain/ethereum/tbtc.go` reservation proposal validators MUST call the real `WalletProposalValidator.validateReservationAnchorProposal` / `validateReservationReanchorProposal` contract functions, not a local approximation. | A signer must reject an invalid proposal using the exact on-chain rule set, not a client-side reimplementation that could drift. | `pkg/chain/ethereum/tbtc.go:482-647`; `WalletProposalValidator.sol:966,1143` | No test found — an explicit `TODO(test-coverage)` comment at `pkg/chain/ethereum/tbtc.go:468-476` records that `ValidateReservationAnchorProposal` has no direct unit test because the simulated-backend infra it needs does not exist yet in `pkg/chain/ethereum`; covered only indirectly via `LocalChain` fakes in task-level tests. |

## 8. Non-functional requirements

### Security (from `feature-spec.md` §14 invariants that hold in M1)

- **NFR-SEC-1.** Claim ≡ anchor MUST hold for every `Active` position at
  every settlement (acceptance and each re-anchor hop write `mintedAmount
  = anchorAmount` together — `ReservationProofs.sol:560,805`).
- **NFR-SEC-2.** Every settled acceptance and re-anchor MUST be a strict
  1-input-1-output Bitcoin transaction, checked at proof time
  (`ReservationProofs.sol`; NFR verified structurally, not by a dedicated
  test grep — covered by `Bridge.ReservationSettlement.test.ts`'s proof
  fixtures).
- **NFR-SEC-3.** A reserved deposit MUST NOT be reachable by
  `DepositSweep` at any point in its life (FR-1).
- **NFR-SEC-4.** Every proof-and-settlement-critical parameter MUST be
  snapshotted into the `ReservationAction` record at request time; a later
  governance parameter change MUST NOT affect an in-flight generation
  (`Reservation.sol:611-628,836-848`).
- **NFR-SEC-5.** A `TimedOut` generation MUST remain late-settleable and
  MUST NOT trigger a second refund (`ReservationProofs.sol:482-548`
  guards this by re-taking, not re-crediting, capacity).
- **NFR-SEC-6.** Fee financing on the settlement path
  (`financeInKindFee`, `repayInKindFeeDebt`) MUST be unconditionally
  callable — never gated by a pause flag or reserve level (FR-16;
  `m1-b-implementation.md` §3).
- **NFR-SEC-7.** State-changing entry points on the router MUST revert
  when called directly (not via the Bridge's delegatecall fallback),
  since the router's own storage is permanently empty
  (`ReservationRouter.sol:82-87`).

### Liveness

- **NFR-LIV-1.** Every M1 housekeeping notification (action timeout,
  stale-deposit release, stranding) MUST be permissionless and MUST carry
  no reward requirement to be callable (US-4, US-10, US-11).
- **NFR-LIV-2.** Re-anchor off a retiring wallet MUST be reachable without
  governance involvement (`privileged` only required for a `Live` source,
  FR-5).
- **NFR-LIV-3.** A timed-out re-anchor MUST be retryable after its cooldown
  without any operator intervention beyond calling the timeout notifier
  (FR-7).
- **NFR-LIV-4.** The below-dust report that lets a retiring, reservation-
  free wallet finish closing MUST be filed automatically by the keep-core
  executor rather than depending on an unprompted external caller (FR-24).

### Upgradeability

- **NFR-UPG-1.** `ReservationRouter` code MUST be replaceable only via a
  full Bridge implementation upgrade (proxy-admin ceremony), never via a
  parameter-governance transaction (FR-18, `architecture.md` §7).
- **NFR-UPG-2.** `ReservationVault` MUST NOT be proxy-upgradeable; it is a
  plain `Ownable` contract with immutable constructor arguments
  (non-goal, §3).
- **NFR-UPG-3.** `BridgeState.Storage` MUST stay append-only: every new
  reservation field decrements `__gap` (currently `39`,
  `BridgeState.sol:502`) by exactly the slots added, verified by a
  storage-layout parity test comparing `Bridge`, `BridgeStub`, and
  `ReservationRouter` (`Bridge.StorageLayout.test.ts`).

### Gas / EIP-170

- **NFR-GAS-1.** `Bridge`'s deployed bytecode MUST stay under the EIP-170
  24,576-byte limit with the reservation surface routed through the
  delegatecall extension. Measured at the M1 code (tbtc-v2
  `reservations-upgrade` @ `9f8f5ef1`, 2026-09-28): `Bridge` at 22,914 B —
  1,662 B of headroom — with `WalletProposalValidator` at 22,176 B and
  `BridgeGovernance` at 21,465 B. Method: clean build (`yarn install
  --frozen-lockfile`, `hardhat compile`, Hardhat 2.29.0, solc 0.8.17);
  `Bridge.sol` compiles under the optimizer override `runs=200` from
  `bridgeCompilerConfig` in `hardhat.config.ts` (default is `runs=1000`,
  which yields 24,835 B for `Bridge` and exceeds the limit — hence the
  override). The historical `#1090`-era figures quoted in `feature-spec.md`
  §2 (22,403 B for `Bridge` at `runs=100`, 4,245 B router) are superseded
  by this measurement; `docs/RESERVATION_CAPS_DEPLOYMENT.md`'s live
  follow-up figure of "~22,870B" is likewise superseded.
- **NFR-GAS-2.** `updateReservationCaps` and `updateReservationParameters`
  MUST be called in that order during initial bootstrap so the Decision-1
  relational check evaluates against real (non-zero) operands rather than
  the zero-disjunct short-circuit (`docs/RESERVATION_CAPS_DEPLOYMENT.md`
  "Deploy Order").

### Observability

- **NFR-OBS-1.** Occupancy changes MUST be observable from event logs
  alone (`ReservationOccupancyChanged`, `Reservation.sol:399-403`) without
  requiring continuous polling of `activeReservationsCount()`.
- **NFR-OBS-2.** keep-core MUST expose `active_reservations_count`,
  `max_active_reservations`, `live_wallets_count`, and
  `wallet_reservations_count` as gauges when reservations are enabled
  (`pkg/clientinfo/performance.go:738-741`, verified registered by
  `pkg/clientinfo/performance_test.go:717-757`).
- **NFR-OBS-3.** keep-core SHOULD expose the vault's `inKindFeeDebtSat`
  and fee-reserve balance as a chain-read gauge. **Not implemented**:
  grepping the keep-core tree for `InKindFeeDebt`/`FeeReserve` returns zero
  matches. Open per §12.

### Operational duties (from `m1-b-implementation.md` §5, re-verified against current code)

- **NFR-OPS-1.** Re-anchor executor on `WalletMovingFunds`, with alerting
  — implemented (FR-23). **Not automated for a `Closing` source**: the
  contract permits it (FR-5, FR-6), but `ReservationReanchorTask` only
  triggers on `StateMovingFunds`; draining a `Closing` wallet's
  reservations reaches capacity only via a permissionless caller, a gap
  open per §12.
- **NFR-OPS-2.** Free-slot / occupancy monitor — implemented (NFR-OBS-2);
  `m1-b-implementation.md` §5's 2026-09-03 note that these gauges were
  deferred is stale against the current keep-core tip.
- **NFR-OPS-3.** Action-timeout watch — implemented (FR-25).
- **NFR-OPS-4.** Stranding watcher on `Terminated`/`Closed` wallets —
  implemented (FR-26). **Not automated** for the third on-chain-eligible
  precondition — a `Closing` wallet past `dissolutionEligibleAt` — which
  has no client trigger and relies on a permissionless caller, a gap open
  per §12.
- **NFR-OPS-5.** Below-dust report after the last re-anchor — implemented
  (FR-24). In-kind fee reserve / debt watch — **not implemented**
  (NFR-OBS-3); this duty from the original list remains an open gap.

## 9. Parameters and caps

All eight `updateReservationParameters` fields and all three
`updateReservationCaps` fields validate on write (FR-12, FR-13). "Launch
value" distinguishes three sources: the **on-chain hard bound** (a
protocol constant no setter can violate), the **local/test default** used
by `97_set_reservation_parameters.ts` (explicitly documented as diverging
from the mainnet posture — never the launch value), and the **mainnet
value**, which `98_generate_reservation_mainnet_calldata.ts` requires as
an environment variable with **no code-level default** (the script throws
if any is missing).

| Parameter | Setter | On-chain validation | Launch value | Retroactivity |
|---|---|---|---|---|
| `reservationTxMaxFee` | `updateReservationParameters` | `> 0` | Mainnet: env `RESERVATION_TX_MAX_FEE_SATS` (no repo default). Local/test: 1000 sats | Snapshotted per action request; never retroactive |
| `reservationMinAmount` | `updateReservationParameters` | `> reservationTxMaxFee` | Mainnet: env `RESERVATION_MIN_AMOUNT_SATS`. Local/test: 10,000 sats | Prospective |
| `reservationTermSeconds` | `updateReservationParameters` | `[MIN_RESERVATION_TERM=90d, MAX_RESERVATION_TERM=730d]` | Mainnet: env `RESERVATION_TERM_SECONDS`, hard-failed below 90d by the calldata script. Local/test: 7,776,000s (90d) | Applies to future term grants only; never alters an existing `expiresAt` |
| `reservationDissolutionDelay` | `updateReservationParameters` | None beyond overflow safety in `expiresAt + delay` | Mainnet: env `RESERVATION_DISSOLUTION_DELAY_SECONDS`. Local/test: 86,400s (1d) | Snapshotted into `dissolutionEligibleAt` per term grant; never retroactive (`Reservation.sol:198-208`) |
| `reservationMaxTotalAmount` | `updateReservationParameters` | Decision-1 relational invariant vs. `maxActiveReservations * reservationMaxSingleAmount` (skipped if either is 0) | Mainnet: env `RESERVATION_MAX_TOTAL_AMOUNT_SATS`. Local/test: 10,000,000 sats | Applies to the next request; does not retroactively affect existing positions |
| `maxReservationsPerWallet` | `updateReservationParameters` | `> 0` | Mainnet: env `RESERVATION_MAX_PER_WALLET`, **hard-enforced == 1** by the calldata script (throws otherwise — `98_generate_reservation_mainnet_calldata.ts:409-414`). Local/test: 5 (documented as deliberately larger, not the launch value) | Prospective |
| `reservationActionTimeout` | `updateReservationParameters` | `> REQUEST_TIMEOUT_SAFETY_MARGIN` (2h) | Mainnet: env `RESERVATION_ACTION_TIMEOUT_SECONDS`. Local/test: 86,400s (1d) | Snapshotted per action request |
| `reservationRenewalWindowSeconds` | `updateReservationParameters` | `> 0` and `< reservationTermSeconds` | Mainnet: env `RESERVATION_RENEWAL_WINDOW_SECONDS`. Local/test: 86,400s | Written for M2 storage-completeness; unread in M1 |
| `maxReservationsAmountPerWallet` | `updateReservationCaps` | None directly (`0` disables the cap) | Mainnet: env `RESERVATION_PER_WALLET_CAP_SATS`. Local/test: 1,000,000 sats | Prospective |
| `reservationMaxSingleAmount` | `updateReservationCaps` | Feeds the Decision-1 invariant | Mainnet: env `RESERVATION_SINGLE_AMOUNT_CAP_SATS`. Local/test: 100,000 sats | Prospective |
| `maxActiveReservations` | `updateReservationCaps` | `> 0` (the M1 launch gate) + Decision-1 invariant | Mainnet: env `RESERVATION_MAX_ACTIVE`. Local/test: 100 | Prospective |
| `initiationFeeBps` (vault) | `updateInitiationFee` (owner) | `<= MAX_FEE_BASIS_POINTS` (500 = 5%) | Constructor default: 40 bps | Applies to future `receiveBalanceIncrease` credits only |
| `feeReserveTarget` (vault) | `updateFeeReserveTarget` (owner) | None | Constructor default: 0 (unset) | Applies instantly to the next `sweepFees` call |

Deploy ordering (`docs/RESERVATION_CAPS_DEPLOYMENT.md` "Deploy Order"):
`beginReservationCapsUpdate`/`finalizeReservationCapsUpdate` **before**
`beginReservationParametersUpdate`/`finalizeReservationParametersUpdate`,
so the Decision-1 check in the parameters finalizer evaluates against
real cap values rather than the pre-launch zero short-circuit.
`setReservationRouter` runs before either, and `setVaultStatus(vault,
true)` runs last (`architecture.md` §8).

## 10. Failure modes

| Failure | Detection | On-chain outcome | Operator action |
|---|---|---|---|
| Acceptance authorization not signed before timeout | `ReservationAcceptanceRequested` with no matching `ReservationAccepted`; action-timeout watcher poll | Capacity released via `notifyReservationAcceptanceTimedOut`; generation stays late-settleable; **no slashing** | Watcher calls automatically (FR-25); depositor may re-request |
| Re-anchor authorization not signed before timeout | Action-timeout watcher poll | Target capacity released, reservation restored to `Active` on the source wallet, `reanchorCooldownUntil` set; **no slashing** | Watcher calls automatically; retry after cooldown |
| Custodying wallet becomes `Terminated` (liveness failure or fraud) while holding an `Active` reservation | Stranding watcher on wallet-closed/terminated events | `notifyReservationStranded` on request; owner's tBTC unaffected, in-kind option lost | Stranding watcher calls automatically (FR-26); no reward, no compensation |
| Custodying wallet is `Closing` and reaches `dissolutionEligibleAt`, or still holds reservations while retiring in `Closing` | **No automated detection for either path**: no keep-core watcher observes a `Closing` wallet crossing `dissolutionEligibleAt` (stranding), and `ReservationReanchorTask` triggers re-anchor only on `StateMovingFunds`, not `StateClosing` (`pkg/tbtcpg/reservation_reanchor.go:139`) | Both `notifyReservationStranded` and `requestReservationReanchor` are contract-eligible for a `Closing` source (FR-5, FR-6, FR-9), but neither is called automatically | No automatic call; anyone may call either entry point manually as a permissionless backstop — open per §12 (`m1-keep-core-readiness/01-gap-analysis.md` Major row) |
| Reserved deposit never accepted before its refund deadline | Stale-deposit watcher poll | `notifyStaleReservedDeposit` clears the pending-deposit slot | Watcher calls automatically; depositor claims the Bitcoin refund path |
| Griefing depositor reveals a reserved deposit and never funds the anchor | Governance monitoring `pendingReservedDeposits` | `forceStaleReservedDeposit` clears it before the refund deadline | Governance calls manually — no automated trigger exists |
| Active-position cap saturation | Occupancy gauges vs. `maxActiveReservations` (alert thresholds 70%/90% per `docs/RESERVATION_CAPS_DEPLOYMENT.md`) | New acceptance requests revert (`"Active reservations cap exceeded"`) | Governance raises `maxActiveReservations` via `updateReservationCaps`, respecting the Decision-1 invariant |
| In-kind fee reserve depleted during re-anchor settlement | **No automated detection** — `inKindFeeDebtSat` has no keep-core gauge (NFR-OBS-3) | `financeInKindFee` never reverts; shortfall recorded as public debt | Anyone may call `repayInKindFeeDebt`; governance should manually watch `InKindFeeFinanced` events until a gauge exists |
| Governance stages an invalid parameter/cap combination | Transaction simulation before submission | `updateReservationParameters`/`updateReservationCaps` revert on the violated `require` | Correct the ordering/values and re-stage per §9 |
| `receiveBalanceIncrease` trusts an unverified Bank-routed credit if a vault is marked trusted before `reservationVault` is wired | Pre-activation sanity checks (`docs/RESERVATION_CAPS_DEPLOYMENT.md` "Verification") | No on-chain guard — this is a documented, not a fixed, gap (`ReservationVault.sol:147-160`) | Governance runbook enforces the correct activation order manually |
| Global or per-wallet capacity saturates with no `Live` target anywhere | Occupancy monitor at 90%+ | Every `requestReservationReanchor` call reverts (`"Wallet reservations cap exceeded"` on every candidate target) | Governance raises caps before saturation; this is the `m1-b-implementation.md` §4.1 launch-gate scenario the caps exist to convert into a revert rather than a stuck wallet |
| Retiring wallet proves funds moved while still holding reservation anchors | No automated detection of the intermediate state itself | Wallet sits in `MovingFunds` indefinitely — not slashable (its clock is deleted on proof), not stuck (re-anchor still works) | Re-anchor executor drains the count, then the below-dust report closes the wallet (FR-23, FR-24) |

## 11. Acceptance and verification

M1's acceptance gate is the full `testing-plan.md` program (pre-audit and
during-audit tooling, effort and critical-path impact — see its §4 before
its §3), a testnet round (`timeline-estimate.md`), and external audit
sign-off before mainnet activation; none of that is duplicated here. This
document's own acceptance criterion is narrower: every FR in §7 traces to
enforcing code that exists at the cited M1 ref and, where a test exists,
to a real test file (§13). `forceStaleReservedDeposit` (part of FR-11) and
FR-28's keep-core proposal-validator dispatch currently have `no test
found` against the searched test directories; NFR-SEC-2 is exercised only
structurally through settlement test fixtures rather than by a dedicated
shape-assertion test. These are gaps for the testing-plan program to
close, not resolved here.

## 12. Open decisions

Items still open against the current M1 code (superset already resolved
is omitted; see `roadmap.md` §7 and `milestone-inventory.md` §7 for the
full D-1..D-27 register and prior resolution history):

- **keep-core client coverage stops at `MovingFunds`/`Terminated`+`Closed`,
  not `Closing`.** Two automated paths do not extend to a `Closing`
  source, even though the Bridge accepts one for both: (1)
  `ReservationReanchorTask` triggers re-anchor only on `StateMovingFunds`
  (`pkg/tbtcpg/reservation_reanchor.go:139`), so a `Closing`-source
  re-anchor is reachable only via a permissionless
  `requestReservationReanchor` call; (2) the stranding watchers trigger
  only on `StateClosed`/`StateTerminated`, so a `Closing` wallet reaching
  `dissolutionEligibleAt` is stranded only via a permissionless
  `notifyReservationStranded` call. Both are contract-safe permissionless
  backstops, not launch-blocking, but they are real operational gaps
  tracked as a Major finding in
  `m1-keep-core-readiness/01-gap-analysis.md` ("Stranding coverage gap
  for `Closing`-past-`dissolutionEligibleAt` wallets"). Whether keep-core
  should extend either task to cover `Closing` is undecided.
- **Restoring re-anchor's eligibility gate at M2.** B deletes the
  `< dissolutionEligibleAt` gate to make re-anchor unbounded (US-5); M2's
  dissolution design must independently decide whether to restore some
  form of it. Deliberately left open (`roadmap.md` §7 item 4, reviewed
  2026-09-07, not re-decided).
- **Structural bound on cumulative re-anchor fee loss as a fraction of the
  claim.** `cumulativeReanchorFee` is tracked but uncapped; which lever
  (per-reservation fractional bound at acceptance, vs. a
  proportional-to-`mintedAmount` dust floor) closes the ratio is
  unresolved (`roadmap.md` §7 item 5).
- **Whether arriving fee revenue floors debt repayment at
  `feeReserveTarget` in `sweepFees`.** `roadmap.md` §7 item 6 records a
  2026-09-07 decision to cap debt repayment at the reserve target so it
  degrades to partial repayment instead of reverting. Re-reading the
  current `sweepFees`/`_burnFromReserve` code (`ReservationVault.sol:286-372`)
  finds no floor at `feeReserveTarget` on the debt-repayment burn (it
  burns up to the full current balance) and no `require(balance >
  feeReserveTarget)` guard either (the function returns early instead of
  reverting when balance doesn't exceed target after repayment). Whether
  this is the intended final shape of the decision or a partially-applied
  fix has not been re-confirmed with the implementer — flagged rather than
  asserted either way.
- **Reservation-eligible wallet allowlist.** `roadmap.md` §7 item 3 records
  a 2026-09-07 decision to add one; grepping the current M1 code finds no
  allowlist mapping, setter, or check anywhere in `ReservationRouter.sol`
  or `Reservation.sol` — not implemented, separate repo/scope per that
  item's own note.
- **`inKindFeeDebtSat`/fee-reserve observability.** No keep-core chain-read
  or gauge exists (NFR-OBS-3); tracked as a real operational gap, not a
  silently dropped one, per the original `m1-b-implementation.md` §5
  2026-09-07 status note, still true against the current tree.
- **Bridge deployed-bytecode re-measurement — resolved 2026-09-28.** The
  EIP-170 margin at the M1 code is now measured (NFR-GAS-1): `Bridge` 22,914
  B, 1,662 B headroom, clean build of `reservations-upgrade` @ `9f8f5ef1`.
  The only remaining open note: any M2 Bridge addition shrinks that
  1,662-B headroom.
- **External-router bytecode-delta spike.** Tracked as non-blocking M2
  debt in `docs/RESERVATION_CAPS_DEPLOYMENT.md` ("Tracked Follow-up") —
  quantify the rejected external-router-with-callbacks alternative
  empirically before M2 adds more reservation surface.
- **Whether a rails release with no reachable in-kind exit is acceptable
  to design partners.** Resolved as an operational/disclosure matter
  (`roadmap.md` §7 item 1, 2026-09-07: "accept as-is, disclose the
  no-exit-until-m2 risk"), not a code change — listed here because it is
  a live commitment a design-partner activation runbook must still act on.
- **M2 migration requirement (design input, not an open M1 item).**
  Confirmed by the project owner on 2026-09-28: M2 will deliver
  redemption/renewal via a new `ReservationVault` deployment plus a
  depositor migration (Option B stands; no pause-flag vault). Verified
  in M1 code: the re-point path
  `updateReservationParameters` only allows changing
  `Bridge.reservationVault` when `reservationTotalAmount == 0 &&
  pendingReservedDeposits == 0` (`Reservation.sol:1300-1317` at
  `9f8f5ef1`), which under variant B is reachable only once every
  position has been stranded/released. That check is Bridge-side library
  code, replaceable by the M2 Bridge upgrade, so the migration requires
  M2's Bridge upgrade to change the vault-binding rule (for example
  per-position vault binding or a governed migration path); the M1 vault
  must keep serving existing positions' settlement paths (e.g.
  `financeInKindFee` on re-anchor) until they migrate. The mechanism is
  an M2 design choice; the requirement is stated here.

## 13. Traceability

| User story | Functional requirement(s) | Code | Test |
|---|---|---|---|
| US-1 | FR-1 | `Deposit.sol:215-238,386-395`; `DepositSweep.sol:432-445` | `Bridge.Deposit.test.ts` |
| US-2 | FR-2, FR-19, FR-20 | `Reservation.sol:487-638,916-958` | `Bridge.ReservationAcceptanceAuthorization.test.ts` |
| US-3 | FR-3 | `ReservationProofs.sol:473-626` | `Bridge.ReservationSettlement.test.ts` |
| US-4 | FR-4 | `Reservation.sol:655-691` | `Bridge.ReservationAcceptanceAuthorization.test.ts` |
| US-5 | FR-5 | `Reservation.sol:722-858` | `Bridge.ReservationSettlement.test.ts` |
| US-6 | FR-5, FR-6 | `Reservation.sol:722-858` | `Bridge.ReservationSettlement.test.ts` |
| US-7 | FR-7 | `Reservation.sol:860-914` | `Reservation.test.ts:1609-1612` |
| US-8 | FR-8 | `ReservationProofs.sol:640-809` | `Bridge.ReservationSourceAnchorBinding.test.ts` |
| US-9 | FR-14 | `Wallets.sol:360-386` | `Bridge.Wallets.test.ts` |
| US-10 | FR-9, FR-10 | `Reservation.sol:985-1115` | `Bridge.ReservationStranding.test.ts`, `Bridge.ReservationStrandingLibrary.test.ts` |
| US-11 | FR-11 | `Reservation.sol:1117-1165` | `Bridge.ReservationStranding.test.ts`, `Bridge.ReservationStrandingLibrary.test.ts` |
| US-12 | FR-11 | `Reservation.sol:1167-1213` | `no test found` |
| US-13 | FR-22 | `pkg/tbtcpg/reservation_acceptance.go:977-1078` | `pkg/tbtcpg/reservation_acceptance_test.go` |
| US-14 | FR-12, FR-13 | `Reservation.sol:1220-1406` | `Bridge.ReservationCaps.test.ts` |
| US-15 | FR-15, FR-16, FR-17 | `ReservationVault.sol:161-339` | `ReservationVault.test.ts` |
| US-16 | FR-28 | `pkg/chain/ethereum/tbtc.go:482-647`; `WalletProposalValidator.sol:966,1143` | `no test found` |
| US-17 | NFR-OBS-1, NFR-OBS-2, NFR-OBS-3, FR-25, FR-26 | `pkg/clientinfo/performance.go:738-741`; `pkg/maintainer/spv/reservation_action_timeout_watch.go`, `reservation_stranding_watch.go` | `pkg/clientinfo/performance_test.go`, `pkg/maintainer/spv/reservation_action_timeout_watch_test.go`, `reservation_stranding_watch_test.go` |
| US-18 | FR-18 | `ReservationRouter.sol:449-597` | `Bridge.ReservationAbiSnapshot.test.ts` |

## 14. Glossary

| Term | Definition | Defined at |
|---|---|---|
| Reservation ("position") | A segregated custody record for one deposit's UTXO, keyed by `reservationKey` | `feature-spec.md` §3.1 |
| Anchor / anchor UTXO | The wallet-controlled Bitcoin output currently holding a reservation's value; changes on every re-anchor hop | `feature-spec.md` §3.1; `architecture.md` §5 |
| Reservation key | `keccak256(fundingTxHash \| fundingOutputIndex)` — doubles as the underlying deposit key | `Reservation.sol:487-490` |
| Designated wallet | The wallet named in a reserved deposit's reveal script; only it may accept that deposit | `Reservation.sol:534-537` |
| Two-phase settlement | The authorize-then-prove pattern: a request reserves capacity and snapshots parameters, a later SPV proof settles it | `feature-spec.md` §1, §4 |
| Action / generation | One nonce-keyed attempt at a Bitcoin-side action against a position (`reservationActions[actionKey(key, nonce)]`) | `Reservation.sol:246-343` |
| Request nonce | Monotonic per-position counter; every action request increments it | `Reservation.sol:182-186` |
| Acceptance | The action type that anchors a revealed reserved deposit and mints tBTC against it | `Reservation.sol:110-116` |
| Re-anchor | The action type that moves an `Active` reservation's anchor to a different `Live` wallet | `Reservation.sol:110-116`; US-5, US-6 |
| Custody | Wallet control of the anchor output; identical trust model to pooled tBTC custody | `vba-technical-diagram.md` §"Components and roles" |
| Claim ≡ anchor | Invariant that `mintedAmount` always equals the current `anchorAmount` | `feature-spec.md` §14 item 1 |
| Segregated custody | A reserved deposit/anchor is never swept into the pooled main UTXO | `feature-spec.md` §14 item 3; FR-1 |
| Stranded | Terminal M1 state for a position whose custodying wallet died before the anchor could move; a bookkeeping write-off, not a fund movement | `exit/stranded.md` §1 |
| Terminated | One-way wallet state reached by a moving-funds timeout, moved-funds-sweep timeout, or fraud-challenge-defeat timeout | `exit/stranded.md` §1 |
| Dissolution | M2-only action type that releases a position after its term and delay; absent from M1 Bridge code | `feature-spec.md` §1; §3 non-goals |
| `dissolutionEligibleAt` | `expiresAt + reservationDissolutionDelay`, snapshotted per term grant; M1's only reader is the stranding precondition for a `Closing` wallet | `Reservation.sol:198-220`; FR-9 |
| Reserved deposit | A revealed deposit routed to the reservation vault, tracked in `pendingReservedDeposit` until accepted or marked stale | `Reservation.sol` §3.3 (`BridgeState.sol` struct) |
| Stale reserved deposit | A reserved deposit whose acceptance window lapsed without settlement; released via `notifyStaleReservedDeposit` or `forceStaleReservedDeposit` | `Reservation.sol:1117-1213`; US-11, US-12 |
| In-kind fee | The Bitcoin miner fee of a re-anchor hop, financed by the vault so total tBTC supply tracks Bitcoin backing | `ReservationVault.sol:199-234`; `m1-b-implementation.md` §3 |
| Fee reserve | The vault's retained initiation-fee balance, up to `feeReserveTarget`, that finances in-kind fees | `ReservationVault.sol:67-78` |
| `inKindFeeDebtSat` | Public, repayable debt recorded when the fee reserve cannot cover a financed fee | `ReservationVault.sol:80-85` |
| Occupancy / active positions | `activeReservationsCount` against the `maxActiveReservations` launch-gate cap | `Reservation.sol:916-930`; NFR-OBS-1/2 |
| Variant A+ / Variant B | The two M1 design options compared in `m1-variant-comparison.md`; B (minimal router, no dissolution) was decided 2026-08-21 | `README.md`; `m1-variant-comparison.md` |
| Option B (vault decision) | The 2026-09-24 decision, confirmed by the project owner on 2026-09-28, that the M1 vault ships minimal and M2 delivers redemption/renewal via a new vault deployment and migration ceremony, not pause flags | `README.md` Decision 4 |
| M1 / M2 | Milestone 1 (this document's scope: create, custody, re-anchor) / Milestone 2 (redemption, renewal, dissolution, veto) | §3, §4 |
| SPV proof | Simplified Payment Verification proof (Merkle inclusion + coinbase + proof-of-work) submitted to settle a Bitcoin-side action | `ReservationProofs.sol:137-471` |
| Router (delegatecall) | `ReservationRouter`, reached via the Bridge's fallback `delegatecall`, executing on Bridge storage with Bridge authority | `ReservationRouter.sol:27-95`; FR-18 |
| EIP-170 | The 24,576-byte Ethereum deployed-bytecode size limit that motivated the router split | `feature-spec.md` §2; NFR-GAS-1 |
| Decision-1 relational invariant | `reservationMaxTotalAmount <= maxActiveReservations * reservationMaxSingleAmount` (skipped if either factor is 0), validated in both governance setters | `Reservation.sol:1215-1232`; §9 |
| Launch gate | `maxActiveReservations > 0`, required before any reservation can be accepted | `Reservation.sol:1383-1385` |
| Deploy-inert / activation | M1 ships with the reservation machinery present but unreachable until governance completes the staged activation sequence | `feature-spec.md` §11; §9 |
| Redemption, renewal, veto, retry credit, partial redemption | M2-only concepts; declared in storage/enums for layout stability but not constructible in M1 | §3 non-goals |

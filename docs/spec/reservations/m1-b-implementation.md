# m1 variant B — Implementation Scope

Status: DRAFT — decided 2026-08-21.

Milestone 1 is **variant B with a minimal router**: an essentials-only rewrite
that ships creation, custody and re-anchoring, and omits dissolution.
`roadmap.md` §1 is the authoritative scope statement; `m1-variant-comparison.md`
§5-§6 holds the argument that was weighed, including the case against B, which
is retained rather than deleted.

This document is the buildable form of that decision: what the router contains,
what the vault must contain, and the gates that must close before activation.

`requirements.md` states the functional requirements (FR-n) this scope
satisfies; `architecture.md` diagrams the flows and state machines it
implements.

---

## 1. Two layers, opposite rules

The decision to minimise the router is safe **because** router code is
replaceable; the same reasoning forbids minimising the vault (`roadmap.md`
§0.7).

| | `ReservationRouter` | `ReservationVault` |
|---|---|---|
| Nature | Bridge code reached by `delegatecall` | Separate contract, plain `Ownable` |
| Replacing it costs | A Bridge implementation upgrade, which m2 needs anyway | Deploying v2 and re-pointing `Bridge.reservationVault` |
| Gate on replacement | Proxy-admin ceremony | `reservationTotalAmount == 0 && pendingReservedDeposits == 0` (`Reservation.sol:1307-1314`) |
| Reachable in B? | Yes | **No in m1**; m2 reaches replacement via the depositor opt-in migration ceremony the project owner confirmed on 2026-09-28 — which the re-point guard above makes reachable only after every position is stranded or released, so the ceremony also needs m2's Bridge upgrade to change the vault-binding rule (§3, §6) |
| m1 posture | **Minimise** | **Minimise** (redemption and renewal ship only in a future, separately-deployed vault — not as paused entry points on this one; see §3) |

The original plan required the vault to ship complete with paused initiation
entry points for redemption and renewal (Section 3). On 2026-09-24 (Option B decision),
the posture was simplified: m1 ships a minimal vault (`4d549e64`), and m2 will
deliver in-kind redemption and renewal by deploying a new vault and conducting
a depositor opt-in migration ceremony. The project owner confirmed this migration on 2026-09-28; see
§6 for the bridge-side requirement it imposes.

## 2. Minimal router surface

The stacked router (`feat/utxo-reservation-guards`, pre-partial-redemption)
has 24 entry points (12 state-changing, 12 views). B removes 6 outright
(§2.3: 4 state-changing, 2 views), splits the single `submitReservationProof`
dispatcher into two proof-submission functions (§2.1, net +1 state-changing),
and adds 3 new entries (§2.4: 2 state-changing, 1 view) — for **22**
(11 state-changing, 11 views), matching the M1 router (`ReservationRouter.sol`,
9f8f5ef1).

### 2.1 Retained — state-changing

| Entry point | M1 line | Why B needs it |
|---|---|---|
| `requestReservationAcceptance` | `:211` | The product's entry gate |
| `requestReservationReanchor` | `:232` | B's **only** unpin path; load-bearing, not optional |
| `submitReservationAcceptanceProof` | `:252` | Settles a pending acceptance; `onlySpvMaintainer`. Replaces half of the stacked router's single `submitReservationProof` dispatcher (§2.3) |
| `submitReservationReanchorProof` | `:278` | Settles a pending re-anchor; `onlySpvMaintainer`. Replaces the other half — there is no combined proof-submission entry point in M1 |
| `notifyReservationActionTimeout` | `:304` | Required cleanup; also the slashing path |
| `notifyStaleReservedDeposit` | `:385` | Releases un-accepted revealed deposits |
| `notifyReservationStranded` | `:414` | B's only position-closing *entry point* (the second closing site is inside re-anchor settlement, §4.3) |
| `updateReservationParameters` | `:356` | Governance; also the vault re-point |
| `updateReservationCaps` | `:434` | Governance; the only safety valve at launch |

### 2.2 Retained — views

`reservationCaps` (`:449`), `walletReservationsAmount` (`:467`),
`walletReservationsCount` (`:478`), `reservationByAnchorUtxo` (`:492`,
subject to §4.3), `reservedDepositWallet` (`:509`), `pendingReservedDeposits`
(`:519`), `reservations` (`:527`), `reservationActions` (`:539`),
`reservationParameters` (`:551`), `reservationRouter` (`:595`).

`walletReservationsCount` and `pendingReservedDeposits` are not optional
conveniences: the first is the free-slot monitor's data source (§5), the second
is read by the vault-swap gate.

There is **no** `walletReservations` view (a per-wallet reservation-key array)
in the M1 router. An earlier draft of this document listed it here as
retained; it is not — see §2.3.

### 2.3 Removed

| Entry point | Stacked line | Reason |
|---|---|---|
| `requestReservedRedemption` | `:262` | Redemption deferred to m2 |
| `notifyReservedRedemptionVeto` | `:366` | Veto is vacuous with no redemptions |
| `extendReservation` | `:382` | Renewal deferred to m2 |
| `requestReservationDissolution` | `:302` | B's defining cut |
| `walletPendingDissolution` | `:600` | Dissolution view |
| `submitReservationProof` | `:322` | Split into `submitReservationAcceptanceProof` / `submitReservationReanchorProof` (§2.1) rather than kept as a single dispatcher |
| `walletReservations` | `:524` | Per-wallet reservation-key enumeration; not carried into M1's router. If this enumeration is genuinely needed, that is a regression to flag, not a documented cut — see `requirements.md` §12 Open decisions |

### 2.4 Added

`activeReservationsCount` (`ReservationRouter.sol:583`) — the global position counter §4.1 requires.
`BridgeState.Storage` today has `liveWalletsCount` (`:253`),
`reservationTotalAmount` (`:378`) and per-wallet counts, but **no global
position count**, so both the storage field and its view are new.

`notifyReservationAcceptanceTimedOut` — a state-changing entry point, added
during review to close a real gap: once `notifyReservationActionTimeout` was
narrowed to cover the Reanchor arm only (§2.1), a pending acceptance whose
designated wallet never signs had no permissionless release path — only the
redundant `notifyReservationActionTimeout` name existed before. This
router-level forwarder releases the reserved capacity and marks the action
`TimedOut`, so the position stays available for a later acceptance request.
Legitimate scope addition, not a spec deviation
(`m1-tbtc-v2-readiness/01-gap-analysis.md`).

`forceStaleReservedDeposit` — a governance-only (`onlyGovernance`) escape
valve absent from the stacked router: force-clears a pending reserved
deposit before its refund deadline, without waiting on
`notifyStaleReservedDeposit`'s permissionless path
(`ReservationRouter.sol:399-404`). Folded into the state-changing count
above (§2's header sentence).

### 2.5 Whether the router is needed at all

Keep it. The question is decidable by compiler and was analysed in
`m1-variant-comparison.md` §5.2: the surface costs `Bridge` roughly 2,244 B
inline by subtraction (not the router's standalone 4,245 B, which double-counts
`Governable`/`Initializable`), against 2,173 B of measured margin — so the
`#1090`-era gap was 71 B, and every PR since added surface. More decisively,
m2 must add ~4,641 production lines and ten-plus entry points, which is
**larger than everything B removed**, so the router is needed back regardless.
Deleting it in m1 buys ~736 lines and costs a re-architecture.

## 3. Vault surface: minimal in m1 (Option B decision, 2026-09-24)

The M1 `ReservationVault` (373 lines) ships with exactly **7 external
functions** plus its constructor — the table below is the complete surface,
not a subset:

| Vault entry point | M1 line | Status | Rationale |
|---|---|---|---|
| `receiveBalanceIncrease` | `:161` | **Active** | The mint callback; `onlyBank` |
| `financeInKindFee` | `:215` | **Active — must not be gated** | On the settlement path. Called from `submitReservationReanchorProof` (`ReservationProofs.sol:758-760`), deliberately *after* `action.state = Settled` is already committed (`:734`) — the external fee-financing call runs last, per the function's own checks-effects-interactions comment (`:753-757`). Re-anchor is B's only unpin, so this is reachable and load-bearing |
| `repayInKindFeeDebt` | `:241` | **Active** | Permissionless burn-down of an over-supply re-anchor can create. Gating it would remove a safety valve while leaving the debt |
| `updateFeeReserveTarget` | `:278` | **Active** | `onlyOwner`; sets the reserve balance below which `sweepFees` stops sweeping |
| `sweepFees` | `:297` | **Active** | `onlyOwner`; repays outstanding `inKindFeeDebtSat` from the current balance first, then sweeps any excess above the reserve target |
| `updateInitiationFee` | `:330` | **Active** | `onlyOwner`; single basis-points parameter |
| `receiveBalanceApproval` | `:343` | **Active — always reverts** | Interface-compliance stub; the vault does not support the balance-approval flow (`"Balance approvals not supported"`) |

**`redeemReservation`, `retryRedeemReservation` and `extendCustody` do not
exist in the M1 vault at all.** They existed during interim development
(`origin/feat/utxo-reservation-backing:solidity/contracts/vault/ReservationVault.sol`
— `redeemReservation:293`, `extendCustody:367`, `retryRedeemReservation:469`)
and were dropped before commit `4d549e64`, which is what M1 ships. Per the
2026-09-24 Option B decision (§1), m2 does **not** add them to this vault
behind a pause flag: it deploys a **new** vault and migrates depositors to
it — confirmed by the project owner on 2026-09-28. One consequence the
Bridge side of m2 must absorb (verified in the M1 code): `updateReservationParameters`
only re-points `Bridge.reservationVault` while `reservationTotalAmount == 0
&& pendingReservedDeposits == 0` (`Reservation.sol` ~:1300-1317), which under
B is reachable only once every position has been stranded or released — so
m2's Bridge upgrade must change the vault-binding rule (mechanism is an m2
design choice, e.g. per-position vault binding or a governed migration
path); until then the m1 vault keeps serving existing positions' settlement
paths (e.g. `financeInKindFee` on re-anchor). This supersedes — and closes,
rather than merely defers — the earlier
plan that the m1 vault must ship the full entry-point surface (redemption,
retry, renewal) paused and ready to unpause (`roadmap.md` §0.7/§1.3/§2.2,
`milestone-inventory.md` D-12/D-13/D-20/D-21, an earlier
`03-followup-pr-spec` `#1111` section, and an earlier `inventory/vault.md`).
A finding proposing to reverse this and restore the paused-flag design is
**rejected**: the vault is not proxy-upgradeable, so restoring that design
after launch is not possible without the same migration ceremony Option B
requires anyway.

**The rule that bounds settlement remains active:** a confirmed Bitcoin spend
must always be able to settle, so functions on the settlement path stay
unconditionally callable. The vault states this itself for the in-kind fee: if
the reserve cannot cover the amount, "the shortfall is recorded as
`inKindFeeDebtSat` and the call still succeeds: a confirmed Bitcoin spend must
never fail to settle because of the reserve level"
(`ReservationVault.sol:209-212`).

Note that this makes m1's fee machinery **live**, not dormant: re-anchor
charges an in-kind miner fee, so the fee reserve, `inKindFeeDebtSat` and
`updateFeeReserveTarget` are all in use from the first re-anchor. Monitoring
the debt belongs in §5.

## 4. Launch gates

These are the residual risks `m1-variant-comparison.md` §5.4 identified.
Under the B decision they are **gates, not tradeoffs**: without them B fails by
arithmetic rather than by an attacker (§5.3 of that document). Four gates are
live — §4.1, §4.3, §4.4, §4.5 — each marked Met or Not met below against the
M1 code. §4.2 documents the Option B decision itself; it is a historical
record, not a fifth live gate.

**Failure-mode framing corrected 2026-08-21.** This previously said the failure
mode is "honest operators slashed and depositors stranded". Slashing is only
the branch where a wallet never proves its funds moved. A wallet that *does*
prove it has its MovingFunds clock deleted (`Wallets.sol:434`) and cannot be
timed out at all (`MovingFunds.sol:594-598`), so honest operators are **not**
slashed. Under saturation that case is worse, not better: the position cannot
close, cannot time out, and cannot be stranded, because stranding needs a
`Terminated` wallet that never arrives. See `roadmap.md` §0.8. (Citations in
this paragraph are pinned to `feat/utxo-reservation-guards`, matching
`m1-variant-comparison.md` §5.3's step-5 correction, and were not re-derived
against the M1 snapshot in this pass — unlike §4.1, §4.3, §4.4 and §4.5
below, which were.)

### 4.1 Global active-position cap, enforced at acceptance — Met

**Implemented and verified**, not outstanding work. B never frees a wallet slot,
so occupancy is monotonic toward `liveWalletsCount x maxReservationsPerWallet`;
without a cap this converts into a silent cliff: at saturation re-anchor has
no target (`Reservation.sol:812-819`, the target-slot check), a wallet
holding anchors cannot finish closing because
`notifyWalletClosingPeriodElapsed` requires a zero reservation count
(`Wallets.sol:381-384`, single site — not repeated elsewhere), its
MovingFunds clock expires, and `notifyWalletMovingFundsTimeout` seizes
operator stake and terminates the wallet (`Wallets.sol:468-491`).

All three pieces this section originally called for are already in the M1
code:

- Acceptance enforces `activeReservationsCount < maxActiveReservations`
  before reserving capacity (`Reservation.sol:925-930`).
- `updateReservationCaps` requires `maxActiveReservations > 0`; the
  function's own doc-comment calls this "the milestone 1 launch gate"
  (`Reservation.sol:1359-1361`, enforced at `:1382-1385`).
- The relational check between the amount cap and slot capacity —
  `reservationMaxTotalAmount <= maxActiveReservations *
  reservationMaxSingleAmount` — is `validateReservationCapsInvariant`
  (`Reservation.sol:1217-1231`), wired into both `updateReservationParameters`
  (`:1330-1334`) and `updateReservationCaps` (`:1392-1396`).

### 4.2 Vault ships minimal (Option B decision, 2026-09-24) — historical record, not a live gate

This section documents the Option B decision itself (§3 has the current,
buildable detail); it is not one of the four live gates (§4.1, §4.3, §4.4,
§4.5). The vault ships minimal in m1 (commit `4d549e64`). What this decision
superseded was the earlier plan that the vault ship the full
redemption/renewal entry-point surface behind pause flags — that plan is
retired, not this one. See §3 for the current vault surface and why the
paused-flag alternative is rejected.

### 4.3 `reservationsByAnchorUtxo` reconciliation — Met

**Corrected 2026-08-21.** This previously read: "`#1091` writes the mapping
(`ReservationProofs.sol:465`), `#1094` writes it again for stranding, and
`#1102` removed it from the merged base in favour of `spentMainUTXOs`. Two
write sites and one removal must be reconciled." The removal does not exist.

`reservationsByAnchorUtxo` is **introduced by `#1091`** and is absent from
`#1088`'s branch entirely (0 hits in `BridgeState.sol` on
`feat/utxo-reservation-core`), so `#1102` — which merged into that branch —
had nothing to remove. `spentMainUTXOs` is a **pre-existing Bridge registry**
that the anchor-consumption path writes into, not a competing index
introduced by `#1102` — in the M1 code that write is
`ReservationProofs.sol:904` (`consumeAnchor`), documented at `:900-903` as
"the existing registry of honestly spent wallet UTXOs".

**Corrected again 2026-09-28, against the M1 code
(`ReservationProofs.sol`/`Reservation.sol`, 9f8f5ef1).** The mapping has one
write site, one rewrite site, and two delete sites — not "two write sites and
no removal" as the previous correction concluded:

- **Written** at acceptance settlement (`ReservationProofs.sol:572`).
- **Rewritten** — pointed at the new anchor, not written a second time by an
  unrelated feature — at each re-anchor settlement, inside
  `settleReanchorAccounting` (`ReservationProofs.sol:799-801`).
- **Deleted** when a reservation strands (`Reservation.sol:1001-1010`, inside
  `strandReservation`) and when an anchor is consumed during proof
  settlement (`ReservationProofs.sol:905`, `consumeAnchor` — the same
  function that writes `spentMainUTXOs`, above).

The earlier correction attributed the re-anchor rewrite site to stranding;
stranding is a delete, not a write. The reconciliation this gate actually
requires is keeping the write, the re-anchor rewrite, and the two deletes
mutually consistent — which they are. **Gate met.**

### 4.4 Keep writing `dissolutionEligibleAt` — Met

Acceptance sets
`dissolutionEligibleAt = expiresAt + reservationDissolutionDelay`
(`ReservationProofs.sol:570`, inside `settleAcceptance`). B deletes its only
reader — re-anchor's `< dissolutionEligibleAt` gate — so an essentials-only
rewrite would naturally drop the write as dead code. **It was not dropped**:
the field's own storage comment states the policy directly — "this field's
only on-chain reader ... has been removed ... It must continue to be written
anyway" (`Reservation.sol:198-213`). Without it, m2's dissolution would have
no eligibility date for any m1-era position, and the snapshot semantics that
keep governance changes non-retroactive ("later governance changes never
move the eligibility time of a term already granted", `Reservation.sol:198-202`)
could not be reconstructed.

The general rule: **storage-complete means written, not merely declared.**

**The deeper reason this matters: in m1 B the custody term has no on-chain
consumer at all.** Every would-be reader of `expiresAt` and
`dissolutionEligibleAt` belongs to functionality this milestone does not ship
at all — redemption's `< expiresAt` check, renewal — or was deliberately
deleted: dissolution's `>= dissolutionEligibleAt` check does not exist
because dissolution itself is cut, and re-anchor's `< dissolutionEligibleAt`
gate was removed to make re-anchor unbounded. So the term is not enforced by
anything in m1; it is a **commitment held in storage for m2 to honour**. The
storage *is* the promise, which makes dropping the write a silent
repudiation of it rather than a code-size optimisation.

### 4.5 Storage layout — Met for `dissolutionEligibleAt`; the dropped fields were never M1-reachable, and M2 can recover equivalents from `__gap`

`feature-spec.md` §11's rule is that no live `ReservationAction` may ever span
a layout change, so every field m2's *Bridge-side* logic will read must exist
at m1. `dissolutionEligibleAt` is the field that rule protects today (§4.4,
Met).

**The rule does not make the redemption/renewal-era fields an earlier plan
assumed mandatory, and Option B is not the reason why.** `maxCumulativeReanchorFee`,
`reservationDissolutionTxMaxFee`, `walletPendingDissolution` and
`reservationRetryCreditActionNonce` were declared in `BridgeState.Storage`
during interim development and then removed as dead/unused prior to the M1
code snapshot verified here — not merely left unwired.
`BridgeState.sol:492-496`'s own storage-gap comment says so directly: this
milestone "dropped `maxCumulativeReanchorFee`, `reservationDissolutionTxMaxFee`,
`walletPendingDissolution`, `reservationRetryCreditActionNonce`,
`reservationMaxBackingFractionBps`, `walletReservationKeys`, and
`walletReservationKeyIndex` as unused/dead"; `BridgeGovernanceParameters.sol:1614-1618`
confirms the same for the two fee fields ("removed as dead/unused before
this milestone shipped ... Reservation parameters accordingly do not include
them"). Neither field was ever read or written by any M1-reachable
`ReservationAction`, so §11's rule has nothing to protect for them today:
dropping a field no in-flight action touches cannot split any action's
layout across a version.

**That is a fact about the Bridge side, and Option B does not change it —
Bridge storage-completeness still matters for m2.** M2's redemption, renewal
and dissolution logic is Bridge code (router and libraries) delivered via
the same proxy-implementation upgrade the router already uses (§2.5): it
reads and writes `BridgeState.Storage`, the struct m1 already uses, not a
separate contract's storage. The 2026-09-24 Option B decision (§3, §4.2)
changes only how m2 delivers *vault*-side redemption and renewal
(`ReservationVault`'s own contract storage, via a new deployment and a
depositor migration ceremony) — it says nothing about what the Bridge
upgrade needs, and does not make the Bridge-side storage-completeness rule
irrelevant. If m2's Bridge-side dissolution/redemption design turns out to
need any of these four fields, they are recoverable there: `__gap` still
has 39 slots after m1's reservation fields (`BridgeState.sol:502`) — shared
budget for every future Bridge upgrade, not reserved specifically for
reservations, but enough headroom that re-declaring a handful of fields at
m2, in append-only order, is the normal case rather than a layout
emergency.

**Status: Met** for `dissolutionEligibleAt` (§4.4). The four dropped fields
are **not needed at m1**: no M1-reachable `ReservationAction` ever read or
wrote them, so §11's rule never required keeping them, and m2 can
re-declare equivalents out of `__gap` in its own Bridge upgrade if its
eventual dissolution/redemption design calls for them — independently of
the Option B vault decision.

## 5. Operational duties

B's duty list is longer than A+'s because nothing closes a position on its
own. Of the ten duties below, four are how a human finds out that a §4 gate
is approaching its limit before it reverts (the free-slot and occupancy
monitors watch §4.1's cap; the stranding watcher and stale-deposit cleanup
operationalize the position-closing/cleanup paths §4.3 depends on); the rest
— the re-anchor executor, the timeout watch, the below-dust report, the
fee-debt watch, the cap-dial runbook and the position-age report — are
independent operational needs with no single corresponding launch gate.

| Duty | Why B specifically | Automated in keep-core (`reservations-epic`, f66f11240)? |
|---|---|---|
| Re-anchor executor on `WalletMovingFunds`, with alerting | The only unpin; failure ends in slashing, not delay | **Partial** — `pkg/tbtcpg/reservation_reanchor.go` (`ReservationReanchorTask`) drains `MovingFunds` sources; the contract also accepts a `Closing` source (§2.1), but the task does not scan for or drain those (`~:139`) |
| Free-slot monitor (`walletReservationsCount < cap` across Live wallets) | Leading indicator of the §4.1 cliff | **Partial** — gauge metric registered (`clientinfo.MetricReservationWalletReservationsCount`, gated on `reservationsEnabled`), but no production code path populates it from a chain read yet |
| Occupancy monitor (`activeReservationsCount` vs `liveWalletsCount x cap`) | Alert well before saturation | **Partial** — same gap: `MetricReservationActiveReservationsCount` / `MetricReservationMaxActiveReservations` / `MetricReservationLiveWalletsCount` are registered, unpopulated in production |
| Action-timeout watch | Pending acceptances and re-anchors approaching `reservationActionTimeout`, since expiry slashes | **Yes** — `pkg/maintainer/spv/reservation_action_timeout_watch.go` |
| Stranding watcher on `Terminated`/`Closed` wallets | Releases capacity; one of B's two close paths | **Partial** — `pkg/maintainer/spv/reservation_stranding_watch.go` triggers only on `StateClosed`/`StateTerminated` (`reservation_wiring.go`'s `OnWalletClosed` subscription and startup scan both gate on those two states); `notifyReservationStranded`'s own precondition also allows stranding a `Closing` wallet's reservation once `now >= dissolutionEligibleAt` (`Reservation.sol:1102-1111`), but no client path triggers that branch — permissionless `notifyReservationStranded` is the backstop |
| **Below-dust report after the last re-anchor** | `notifyMovingFundsBelowDust` (`MovingFunds.sol:603`) is the **only remaining** route to close a wallet that proved its funds moved while still holding anchors: the Bridge's own automatic closing attempt inside `notifyWalletFundsMoved` (`Wallets.sol:405-439`) runs once, while the reservation count is still non-zero, and cannot itself be retried | **Yes** — `pkg/tbtcpg/reservation_reanchor.go`'s `ReservationReanchorTask` calls `notifyMovingFundsBelowDustIfEligible` (`:664-730`) after re-anchoring a `MovingFunds` wallet's reservations to zero, invoking `NotifyMovingFundsBelowDust` (test: `TestReservationReanchorTask_Run_NotifiesMovingFundsBelowDust`). Still an operator duty if the reservation task is disabled via `tbtc.Config.ReservationsEnabled` |
| Stale reserved-deposit cleanup | `notifyStaleReservedDeposit` | **Yes** — `pkg/maintainer/spv/reservation_stale_deposit_watch.go` |
| In-kind fee reserve and `inKindFeeDebtSat` watch | Re-anchor charges an in-kind miner fee (§3), so the reserve depletes and can enter debt in m1. Non-zero debt means the system is over-supplied by exactly that amount, publicly visible and repayable by anyone | **No** — zero references to `InKindFeeDebt` or `FeeReserve` anywhere in keep-core's `pkg/`; no chain-read accessor or gauge exists |
| Cap-dial runbook | Trigger, executor and accepted blast radius agreed **before** launch | Manual by nature — a runbook, not code |
| Position-age report | Nothing closes, so age is the only proxy for accumulating permanent liability | **No** — not found in keep-core |

Note what B does **not** need: a dissolution executor. That saving is the
decision's operational upside, and it is real — but it is a saving of
~300-500 production Go lines against the duties above.

**Implementation status (2026-09-03; updated 2026-09-28 against keep-core
f66f11240).** The free-slot and occupancy gauges have progressed since the
2026-09-03 review but are still not complete. `pkg/clientinfo/performance.go`
now declares and registers the four leading-indicator gauge metrics
(`MetricReservationActiveReservationsCount`,
`MetricReservationMaxActiveReservations`, `MetricReservationLiveWalletsCount`,
`MetricReservationWalletReservationsCount`), gated behind
`Config.ReservationsEnabled` so a non-reservation deployment's metric surface
is unchanged — but no production call site sets their values from a chain
read; only the test suite exercises `SetGauge` for them directly. The chain
interface the gauges would read from already exists
(`ActiveReservationsCount() (count uint32, maxActive uint32, err error)` on
`pkg/tbtcpg/chain.go:250`, already consumed by the acceptance task's own cap
check at `pkg/tbtcpg/reservation_acceptance.go:386`), so the remaining work
is periodic-poll wiring from that interface (or a per-wallet equivalent for
the wallet-level pair) into `PerformanceMetrics`, not a new chain read.
Tracked as an open item, not a silently-dropped one.

**Implementation status (2026-09-07; confirmed still open 2026-09-28).** The
in-kind fee reserve and `inKindFeeDebtSat` watch has no recorded deferral
decision, unlike the free-slot/occupancy monitors above: a repo-wide search
still finds zero references to `InKindFeeDebt` or `FeeReserve` anywhere in
`pkg/`. The 2026-09-07 policy question this duty exists to cover — whether
fee revenue should repay `inKindFeeDebtSat` before sweeping — is now
**resolved on-chain**: `sweepFees` repays outstanding debt from the vault's
current balance before computing the sweepable excess
(`ReservationVault.sol:297-319`; decision recorded in `timeline-estimate.md`
§7 item 6). What remains unbuilt is the keep-core-side **observability** — a
chain-read accessor for the reserve/debt balance and a gauge, mirroring the
shape of the free-slot/occupancy gauges above — deferred to the same
follow-up PR for the same reason: it should get its own chain interface and
tests, not be bolted onto an unrelated review-fix pass.

## 6. What m2 must then build

~4,641 production Solidity lines: whole redemption, renewal, veto integration
and their storage (~3,035), `#1096`'s partial redemption (~696), and
dissolution restored (~910). keep-core gains **two** action types, Redemption
and Dissolution, where A+ would have needed only Redemption. The project owner confirmed on 2026-09-28 that m2 will deliver the new-vault migration; no pause-flag vault is in scope.

Two inherited decisions m2 must make that A+ would not have created, plus one
hard requirement the new-vault migration imposes (§3, confirmed 2026-09-28):

1. **Whether to restore re-anchor's eligibility gate.** B deletes
   `< dissolutionEligibleAt` (`Reservation.sol:785-788`) to make re-anchor
   unbounded, which is what makes dropping dissolution sound. Restoring
   dissolution does not automatically restore the gate, and leaving it out
   means a position can be rotated indefinitely past its eligibility date.
2. **Whether m1-era positions get the m2 semantics.** They will carry
   `dissolutionEligibleAt` values snapshotted under m1 parameters (§4.4), and
   `Reservation.sol:198-202` makes those non-retroactive by design.
3. **The vault-binding rule on the Bridge.** `updateReservationParameters`
   only re-points `Bridge.reservationVault` while `reservationTotalAmount == 0
   && pendingReservedDeposits == 0` (`Reservation.sol` ~:1300-1317), which under
   B is reachable only once every position has been stranded or released — so
   m2's Bridge upgrade must change the vault-binding rule to make the
   migration ceremony actually reachable (mechanism is an m2 design choice,
   e.g. per-position vault binding or a governed migration path). Until m2
   ships, the m1 vault keeps serving existing positions' settlement paths
   (e.g. `financeInKindFee` on re-anchor).

The milestone ratio is worth stating plainly: 5,261 : 4,641, or **1.13 : 1**.
Against A+'s 1.65 : 1 and the stacked plan's 13.2 : 1, B's split produces two
comparable projects rather than a large first release and a small follow-up
(`timeline-estimate.md` §7).

---

## Provenance

Derived 2026-08-21 from the variant B decision, `roadmap.md` §0.7/§1 and
`m1-variant-comparison.md` §5.2-§6. Source-verified on
`feat/utxo-reservation-guards` unless noted: router surface
(`ReservationRouter.sol:242-643`), vault surface
(`ReservationVault.sol:222-568`), vault re-point gate
(`Reservation.sol:1263-1274`), re-anchor target cap (`:820-827`), re-anchor
eligibility gate (`:785-788`), acceptance's `dissolutionEligibleAt` write
(`ReservationProofs.sol:537-539`), wallet closing preconditions
(`Wallets.sol:674-677`, `:707-709`), MovingFunds timeout slashing (`:493-523`),
storage fields (`BridgeState.sol:253`, `:378`), once-only router setter
(`:1018-1021`). Bytecode figures are `#1090`-era, quoted from `feature-spec.md`
§2 and not re-measured.

**Re-verified 2026-09-28 against the M1 code snapshot** (tbtc-v2
`reservations-upgrade` @ `9f8f5ef1`; keep-core `reservations-epic` @
`f66f11240`): the router surface (§2, 22 entries), vault surface (§3, 7
functions), the vault re-point gate (§1, `Reservation.sol:1307-1314`), the
re-anchor target-slot check (§4.1, `:812-819`), the wallet-closing
precondition (§4.1, `Wallets.sol:381-384`), MovingFunds timeout slashing
(§4.1, `Wallets.sol:468-491`), the `dissolutionEligibleAt` write (§4.4,
`ReservationProofs.sol:570`), the `reservationsByAnchorUtxo`
write/rewrite/delete sites (§4.3), the dropped storage fields (§4.5,
`BridgeState.sol:492-496`), and keep-core's watcher/executor/gauge coverage
(§5) were all re-derived directly against that snapshot; citations elsewhere
in this document (§4's opening failure-mode paragraph, §2.5's byte-cost
figures) remain pinned to `feat/utxo-reservation-guards` / `#1090`-era
measurements and were not re-verified in this pass.

**Baseline caveat (2026-08-21).** The subtraction assumes `#1090` moved out
only the reservation surface. If it also refactored unrelated `Bridge` code, the
surface's inline cost is smaller and the 71 B gap is not the real one
(`feature-spec.md` §2).

A scope decision, not a commitment of dates.

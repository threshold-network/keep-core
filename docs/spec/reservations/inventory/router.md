# Router and Bridge-integration inventory

Source-verified on `solidity/` at the `feat/utxo-reservation-guards` tip
(`#1094`). Note the
caveat in `pr-map.md` §3: this tip does not contain the `#1102` fold, so
`BridgeState.sol` line numbers here predate those fixes.

## 1. Router entry points: 24, independently counted

`ReservationRouter.sol` declares **24 functions, all `external`**. There are no
internal helpers and no public functions. The count and every line number below
were enumerated mechanically from the file, and they match
`m1-b-implementation.md` §2 exactly.

### Retained in m1 - state-changing (8)

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `requestReservationAcceptance` | `ReservationRouter.sol:242` | #1091 | entry-point | yes | yes | The product's entry gate |
| `requestReservationReanchor` | `:286` | #1091 | entry-point | yes | yes | Variant B's only unpin path; load-bearing |
| `submitReservationProof` | `:322` | #1091 | entry-point | yes | yes | Only `onlySpvMaintainer` entry point; m1 dispatches acceptance and re-anchor only |
| `notifyReservationActionTimeout` | `:351` | #1091 | entry-point | yes | yes | Required cleanup, and the slashing path |
| `notifyStaleReservedDeposit` | `:450` | #1094 | entry-point | yes | yes | Releases un-accepted revealed deposits |
| `notifyReservationStranded` | `:461` | #1094 | entry-point | yes | yes | Variant B's only position-closing path |
| `updateReservationParameters` | `:421` | #1088 | entry-point | yes | yes | `onlyGovernance`; also carries the vault re-point |
| `updateReservationCaps` | `:476` | #1093 | entry-point | yes | yes | `onlyGovernance`; the only safety valve at launch |

### Retained in m1 - views (11)

| Item | Source | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|
| `reservationCaps` | `:487` | view | yes | yes | Cap readback |
| `walletReservationsAmount` | `:503` | view | yes | yes | Per-wallet exposure |
| `walletReservationsCount` | `:514` | view | yes | yes | **Load-bearing**: the free-slot monitor's data source |
| `walletReservations` | `:524` | view | yes | yes | Per-wallet key list |
| `reservationByAnchorUtxo` | `:538` | view | yes | yes | Reverse index reader; see the two-write-site hazard |
| `reservedDepositWallet` | `:555` | view | yes | yes | Reveal-time binding readback |
| `pendingReservedDeposits` | `:565` | view | yes | yes | **Load-bearing**: read by the vault re-point gate |
| `reservations` | `:573` | view | yes | yes | Position readback |
| `reservationActions` | `:585` | view | yes | yes | Action readback |
| `reservationParameters` | `:609` | view | yes | yes | Parameter readback |
| `reservationRouter` | `:640` | view | yes | yes | Router address readback |

### Removed in m1 (5)

| Item | Source | PR | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|---|
| `requestReservedRedemption` | `:262` | #1091 | entry-point | no | yes | Redemption deferred |
| `requestReservationDissolution` | `:302` | #1091 | entry-point | no | yes | Variant B's defining cut; note it has **no modifier**, so it is permissionless |
| `notifyReservedRedemptionVeto` | `:366` | #1091 | entry-point | no | yes | Veto is vacuous with no redemptions |
| `extendReservation` | `:382` | #1092 | entry-point | no | yes | Renewal deferred |
| `walletPendingDissolution` | `:600` | #1091 | view | no | yes | Dissolution view |

### Added in m1 (1)

| Item | Source | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|
| `activeReservationsCount` | new | view | yes | yes | Global position counter; see §3 for why it is genuinely new |

**Arithmetic: 24 - 5 + 1 = 20.** Confirmed independently.

## 2. The four delegatecall invariants - all genuinely test-asserted

`epic-merge-plan.md` §3 step 2 asked reviewers to "verify the tests actually
assert them, not just describe them". Answer: **all four have real tests** in
`test/bridge/ReservationRouter.test.ts` (634 lines).

| Invariant | Mechanism | Asserted by |
|---|---|---|
| Storage parity | Router and Bridge share storage-bearing bases in the same order; router additions append to `BridgeState.Storage` with a matching `__gap` decrement | `"should consume exactly fourteen slots from the deployed Bridge gap"` (`ReservationRouter.test.ts:145`) |
| Selector disjointness | `Bridge.fallback` only sees unmatched selectors, so the router must never declare a function Bridge declares | `describe("selector disjointness")` (`:374`), plus `"should revert unknown selectors without executing anything"` (`:542`) |
| No standalone authority | Router runs on its own empty storage when called directly: governance unset, no Bank balance, so every state-changing entry point reverts | `describe("standalone router hardening")` (`:567`), `"should have empty storage of its own"` (`:577`), `"should reject state-changing calls made directly"` (`:587`) |
| One-time setter | `setReservationRouter` reverts once set, keeping router-code changes on the proxy-admin ceremony rather than parameter governance | `describe("setReservationRouter")` (`:411`), `context("when called on a bridge with the router already set")` (`:412`), and two `"should revert without consuming the one-time slot"` cases (`:450`, `:465`) |

The setter itself: `BridgeState.sol:1014-1026`, requiring
`self.reservationRouter == address(0)` (`:1018-1021`), non-zero address
(`:1023-1026`), and deployed code at the target (per its natspec at `:1005-1007`).

**m1 verdict: all four are m1 invariants and all four tests are extractable
as-is.** This is the clearest example of review effort that should not need
re-doing from zero.

## 3. `BridgeState.Storage` reservation fields

| Item | Source | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|
| `reservationMinAmount` | `BridgeState.sol:354` | storage | yes | yes | param |
| `reservationTermSeconds` | `:358` | storage | yes | yes | param; the promise clock, with no on-chain consumer in m1 |
| `reservationVault` | `:361` | storage | yes | yes | The re-point target |
| `reservationTxMaxFee` | `:365` | storage | yes | yes | param; no proposed value yet |
| `reservationDissolutionDelay` | `:372` | storage | yes | yes | param; feeds the `dissolutionEligibleAt` write m1 must keep |
| `reservationMaxTotalAmount` | `:375` | storage | yes | yes | Global amount cap |
| `reservationTotalAmount` | `:378` | storage | yes | yes | Global amount in use; read by the re-point gate |
| `maxReservationsPerWallet` | `:380` | storage | yes | yes | Per-wallet slot cap |
| `reservations` | `:384` | storage | yes | yes | Position records |
| `reservationsByAnchorUtxo` | `:388` | storage | yes | yes | Reverse index; two write sites |
| `walletReservationsCount` | `:391` | storage | yes | yes | Per-wallet slot occupancy |
| `reservationRouter` | `:404` | storage | yes | yes | Router address, one-time settable |
| `reservationActionTimeout` | `:409` | storage | yes | yes | param |
| `reservationRenewalWindowSeconds` | `:415` | storage | yes | yes | param; **written but unread in m1** since renewal is deferred |
| `reservationActions` | `:423` | storage | yes | yes | Action records |
| `walletReservationsAmount` | `:435` | storage | yes | yes | Per-wallet amount |
| `maxReservationsAmountPerWallet` | `:438` | storage | yes | yes | Per-wallet amount cap |
| `reservationMaxSingleAmount` | `:441` | storage | yes | yes | Single-position cap |
| `walletReservationKeys` | `:446` | storage | yes | yes | Per-wallet key list |
| `walletReservationKeyIndex` | `:449` | storage | yes | yes | Key list index |
| `__gap` | `:463` | storage | yes | yes | `uint256[34]` at this tip; 14 slots consumed per the parity test |

Adjacent pre-existing fields the feature depends on:
`liveWalletsCount` (`:253`) and `spentMainUTXOs` (`:336`).

### Confirmed: there is no global active-position counter

Grepping `BridgeState.sol` for `activeReservationsCount`, `reservationsCount` and
`totalReservations` returns **nothing**. Storage has a global *amount*
(`reservationTotalAmount`, `:378`) and per-wallet *counts*
(`walletReservationsCount`, `:391`), but no global count of open positions.

This confirms `m1-b-implementation.md` §2.4 and §4.1: both the storage field and
its view are genuinely new work, not a view over an existing counter. It is the
gate that converts variant B's saturation cliff into a revert, so it is m1 work
that has no extraction source.

## 4. EIP-170 figures

The measured figures in the doc set (`Bridge` at 24,647 B over a 24,576 B limit
before the router fix; 22,403 B after, at `runs=100`; router standalone 4,245 B;
roughly 2,244 B inline by subtraction) are quoted from `feature-spec.md` §2 and
are `#1090`-era. **I did not re-measure them**, and re-measuring requires a
compiler run, which is out of scope for an inventory pass. Treat them as
`UNVERIFIED` at the current tip. Every PR since `#1090` added surface, so the
margin is smaller now than those numbers suggest.

## 5. Reservation test inventory (extraction value)

| File | Lines | Covers |
|---|---|---|
| `Bridge.Reservation.test.ts` | 3717 | Core lifecycle |
| `Bridge.ReservationSettlement.test.ts` | 3258 | Two-phase machine |
| `Bridge.ReservationGuards.test.ts` | 959 | Binding, stranding, pending-deposit guard |
| `Bridge.ReservationBacking.test.ts` | 748 | Claim-equals-anchor, in-kind fee |
| `ReservationRouter.test.ts` | 634 | The four delegatecall invariants |
| `Bridge.ReservationInvariants.test.ts` | 496 | Feature-level invariants |
| `Bridge.StorageLayout.test.ts` | 96 | Append-only layout parity |

9,908 lines of reservation test code exist. The m1 rewrite should treat these as
the primary extraction asset, not the production code: tests encode the reviewed
intent, and the ones covering m1 behaviour transfer with far less rework than the
implementation does.

## Open questions

1. **DECISION NEEDED: does `reservationByAnchorUtxo` the view stay in m1 while
   the reverse index has two unreconciled write sites?** The view is cheap, but
   exposing a half-reconciled index is worse than not exposing it. Reconcile
   first, then decide.
2. **DECISION NEEDED: is `reservationRenewalWindowSeconds` validated in m1?**
   It is a stored parameter with no m1 reader, but `updateReservationParameters`
   validates it relationally against the term. Keep the validation (cheap, and
   it protects m2's semantics) or drop it as dead code?
3. **UNVERIFIED: the EIP-170 margin at the current tip.** Needs one compiler run
   at `runs=100`. This is the only genuinely open question about whether a
   minimal router is even necessary.

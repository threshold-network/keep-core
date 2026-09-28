# ReservationVault Surface Inventory

## Status vs M1 code (2026-09-28)

This fragment was source-verified against `feat/utxo-reservation-guards` (the `#1094` tip) and describes the **pre-Option-B, pre-minimisation** `ReservationVault.sol` (662 lines): a full-featured vault with in-kind redemption, renewal, and guardian pause machinery. **M1 does not ship this vault.**

The M1 code (`reservations-upgrade`@`9f8f5ef1`, commit `4d549e64`) ships a different, minimal 373-line vault: 7 external functions + constructor, no `redeemReservation`, no `extendCustody`, no pause/renewal/guardian machinery, no `redemptionsPaused`. This is the 2026-09-24 "Option B" decision (`../milestone-inventory.md` decision register: M1 ships the minimal vault as-is; M2 delivers redemption and renewal via a **new vault deployment plus a depositor migration ceremony**, not via unpausing flags on this vault - superseding the "ship every entry point behind a pause flag" rule this fragment's old §5 conclusion and §6 activation steps were written against).

See "M1 vault surface" immediately below for the vault M1 actually ships. Everything from "Full storage and events inventory" downward, and the numbered §1-§6 above it, describe the earlier full-featured design and are kept as the design reference for M2's new vault deployment, not as a description of M1.

## M1 vault surface (reservations-upgrade @ 9f8f5ef1)

**Contract:** `contracts/vault/ReservationVault.sol`, 373 lines. Declaration (L46): `contract ReservationVault is IVault, IReservationFeeFinancer, Ownable` - same inheritance as the full vault below (still no dedicated `IReservationVault` interface; `IReservationFeeFinancer` supplies `financeInKindFee`, `IVault` supplies `receiveBalanceIncrease`/`receiveBalanceApproval`). Not proxy-upgradeable: plain `Ownable`, a real constructor, immutable `bank`/`tbtcVault`/`tbtcToken`/`bridge` (L58-61) - the upgradeability analysis in the old §5 below still holds for this vault.

7 external functions + constructor:

|Item|Line|Access|Classification|Note|
|---|---|---|---|---|
|`constructor`|L107-136|deploy-time only|-|Sets immutable `bank`/`tbtcVault`/`tbtcToken`/`bridge`; `initiationFeeBps = 40` (L135). No `renewalsPaused` default - the flag no longer exists.|
|`receiveBalanceIncrease`|L161|`onlyBank` (L164, modifier L102-105)|initiation-path|Mints gross TBTC per depositor from the Bridge-proven anchor credit, charges `initiationFeeBps`, retains the fee in the vault as reserve (L194-196). No vault-internal pause flag gates it; the Bridge's vault-trust status (`setVaultStatus`) is the activation boundary (deploy script 97, step 3).|
|`financeInKindFee`|L215|`require(msg.sender == address(bridge))` (L216)|settlement-path, must never revert|Burns TBTC from the vault's balance to fund a settled re-anchor's miner fee; uncovered shortfall is recorded as `inKindFeeDebtSat` (L222-230) rather than reverting.|
|`repayInKindFeeDebt`|L241|permissionless|settlement-path adjunct|Anyone burns TBTC (capped at outstanding debt) to reduce `inKindFeeDebtSat` (L241-266).|
|`updateFeeReserveTarget`|L278|`onlyOwner`|accounting-path|Governance sets `feeReserveTarget`, the TBTC balance retained before `sweepFees` may move the excess out (L278-284).|
|`sweepFees`|L297|`onlyOwner`|accounting-path|Repays outstanding `inKindFeeDebtSat` first (L300-308), then sweeps the balance above `feeReserveTarget` to `recipient` (L310-318).|
|`updateInitiationFee`|L330|`onlyOwner`|accounting-path|Single-parameter setter for `initiationFeeBps`, capped at `MAX_FEE_BASIS_POINTS` (500 bps, L331-334). Not the old 3-parameter `updateFees` - `extensionFeeBps`/`redemptionFeeBps` no longer exist to update.|
|`receiveBalanceApproval`|L343|anyone, always reverts|accounting-path (stub)|`external pure override`; required by `IVault`/`IReceiveBalanceApproval`, reverts unconditionally (`"Balance approvals not supported"`).|

**Not present in M1:** `redeemReservation`, `retryRedeemReservation`, `extendCustody`, `pauseRenewals`/`unpauseRenewals`, `blockRenewal`/`unblockRenewal`, `setRenewalGuardian`, `renewalsPaused`, `renewalBlocked`, `renewalGuardian`, the 3-parameter `updateFees`, `extensionFeeBps`, `redemptionFeeBps`. All were removed by the Option B minimisation (commit `4d549e64`); none are gated by a pause flag because none are deployed at all.

**In-kind fee economy as implemented:**
- `initiationFeeBps` (`uint16`, L66) - initiation fee on the gross anchored amount, default 40 bps (L135), settable via `updateInitiationFee`, capped at `MAX_FEE_BASIS_POINTS` (500 bps, L56). Charged and retained in `receiveBalanceIncrease`.
- `feeReserveTarget` (`uint256`, L78) - TBTC (18-decimal) balance the vault retains as reserve before `sweepFees` will move anything out; governance-set via `updateFeeReserveTarget`.
- `inKindFeeDebtSat` (`uint64`, L85) - outstanding satoshi debt from re-anchor miner fees the reserve couldn't cover at settlement time; accrued by `financeInKindFee`, reduced by `repayInKindFeeDebt` or automatically inside `sweepFees` before the sweep computation.
- Internal `_burnFromReserve` (L354) is the shared burn primitive behind both `financeInKindFee` and `sweepFees`'s debt-first repayment; the interaction between the two is the subject of the "Finding 2026-09-07" note preserved in Open Questions item 7 below (`sweepFees` can become unconditionally uncallable once `inKindFeeDebtSat` exceeds `feeReserveTarget`; fix not yet implemented).
- Dissolution fee financing does not exist in the M1 code at all (dissolution itself is not implemented in M1 - decision 3, 2026-09-07), so there is no dormant dissolution call path to track, unlike the old §4 below which described one as m2-only-but-present.

---

*Everything from here down (the numbered §1-§6, the storage/events appendix, and the open questions) is the pre-Option-B, pre-minimisation full vault, source-verified against `feat/utxo-reservation-guards` (#1094 tip). It does not describe what M1 ships; it is kept as the design reference for M2's new vault deployment.*

**Contract:** `contracts/vault/ReservationVault.sol` (662 lines)

*"Established fact N" refers to the numbered m1/m2 assignment rules in
`../milestone-inventory.md` §1.2.*
**Declaration:** `contract ReservationVault is IVault, IReservationFeeFinancer, Ownable` (line 57)
**No `IReservationVault` interface exists.** The vault exposes its full public ABI directly. The only dedicated reservation interface is `IReservationFeeFinancer.sol` (the `financeInKindFee` hook). `IVault.sol` supplies `receiveBalanceIncrease` (and inherits `receiveBalanceApproval` from `IReceiveBalanceApproval`).

**Path classification key:**
- **initiation-path:** a function that starts a reservation lifecycle or charges the initiation/extension fee.
- **settlement-path:** a function invoked when a confirmed Bitcoin transaction settles; must never revert because the Bitcoin spend is already confirmed.
- **accounting-path:** bookkeeping, fee management, pause/guardian policy, and sweeps; governance or guardian bookkeeping, not user lifecycle.

**m1/m2 key:**
- `yes` = present and active.
- `flagged` = present but initiation-disabled behind a pause flag (established fact 1 in `../milestone-inventory.md` §1.2: ship every entry point, disable only initiation).
- `no` = omitted in that milestone.

**PR origin:** determinable PR numbers from the stated range (`#1088`, `#1090`-`#1096`, `#1102`); `?` where not determinable from source alone.

---

## 1. Entry-point inventory

|Item|Source|PR|Kind|m1|m2|Note|
|---|---|---|---|---|---|---|
|`receiveBalanceIncrease` L234 `onlyBank`|ReservationVault.sol:234|?|entry-point|flagged|yes|Bank calls it when Bridge proves an anchor and credits gross sats; initiation-path, so must ship in m1 but initiation can be gated by leaving the vault untrusted in the Bridge|
|`redeemReservation` L293 external|ReservationVault.sol:293|?|entry-point|yes|yes|Owner surrenders gross TBTC plus redemption fee, vault unmints and asks Bridge to spend the anchor outpoint; settlement-path, must never revert once called|
|`extendCustody` L367 external|ReservationVault.sol:367|?|entry-point|flagged|yes|Owner pays extension fee for one more term; guarded by `renewalsPaused` and `renewalBlocked` at L388-389, so the pause flag already covers it; renewal is initiation of a new term|
|`pauseRenewals` L409 `onlyGuardianOrOwner`|ReservationVault.sol:409|?|entry-point|yes|yes|Restrictive policy setter; accounting-path; ships in m1 because it is the existing pause pattern to copy for redemption|
|`unpauseRenewals` L415 `onlyOwner`|ReservationVault.sol:415|?|entry-point|yes|yes|Restorative setter; accounting-path; owner-only|
|`blockRenewal` L424 `onlyGuardianOrOwner`|ReservationVault.sol:424|?|entry-point|yes|yes|Per-reservation restrictive setter; accounting-path|
|`unblockRenewal` L432 `onlyOwner`|ReservationVault.sol:432|?|entry-point|yes|yes|Per-reservation restorative setter; accounting-path|
|`setRenewalGuardian` L440 `onlyOwner`|ReservationVault.sol:440|?|entry-point|yes|yes|Replaces the guardian; accounting-path; zero address is deliberate|
|`retryRedeemReservation` L469 external|ReservationVault.sol:469|?|entry-point|yes|yes|Owner re-requests redemption from Bank balance after wallet-fault timeout; settlement-path, fee already collected so no re-charge|
|`financeInKindFee` L529 `external override` (Bridge-only)|ReservationVault.sol:529|?|entry-point|yes|yes|Bridge calls during re-anchor and dissolution settlement; settlement-path, must never revert; LIVE in m1 because re-anchor is a m1 essential|
|`repayInKindFeeDebt` L568 external (permissionless)|ReservationVault.sol:568|?|entry-point|yes|yes|Anyone burns TBTC to reduce recorded in-kind fee debt; settlement-path adjunct; permissionless|
|`updateFeeReserveTarget` L599 `onlyOwner`|ReservationVault.sol:599|?|entry-point|yes|yes|Governance sets the fee reserve target; accounting-path; required activation step per deploy script|
|`sweepFees` L613 `onlyOwner`|ReservationVault.sol:613|?|entry-point|yes|yes|Owner sweeps fee revenue above the reserve target; accounting-path|
|`updateFees` L629 `onlyOwner`|ReservationVault.sol:629|?|entry-point|yes|yes|Governance updates the three fee parameters; accounting-path|
|`receiveBalanceApproval` L655 `external pure override`|ReservationVault.sol:655|?|entry-point|yes|yes|Always reverts; required by `IVault`/`IReceiveBalanceApproval`; accounting-path (stub)|
|`constructor` L183|ReservationVault.sol:183|?|entry-point|yes|yes|Sets immutable Bank/TBTCVault/Bridge, default fees 40/20/20, `renewalsPaused = true`; not an external entry point after deploy but listed for completeness|

**Entry-point count: 16** (15 external functions + constructor).

---

## 2. Path classification and must-never-revert evidence

### Settlement-path functions (must never revert)

**`financeInKindFee` (L529)** — the core settlement hook.

> Natspec (L520-528): "Finances an in-kind Bitcoin miner fee of a settled re-anchor or dissolution transaction... Called by the Bridge during settlement."
>
> L526-528: "If the reserve cannot cover the full amount, the shortfall is recorded as `inKindFeeDebtSat` and the call still succeeds: a confirmed Bitcoin spend must never fail to settle because of the reserve level."

Code evidence: L530 `require(msg.sender == address(bridge), "Caller is not the Bridge")`. The only access check is caller identity; there is no pause check, no reserve-level check that reverts. If the reserve cannot cover, L551-556 records the shortfall as debt rather than reverting. This is the settlement invariant established fact 2 protects.

The `IReservationFeeFinancer` interface natspec (IReservationFeeFinancer.sol:25-30) reinforces this:

> "If the reserve cannot cover the full amount, the shortfall is recorded as public debt and the call still succeeds -- a confirmed Bitcoin spend must never fail to settle because of the reserve level."

The Bridge calls this from `submitReservationReanchorProof` (ReservationProofs.sol:874) and `submitReservationDissolutionProof` (ReservationProofs.sol:995). Re-anchor is a m1 essential (custody wallet rotation), so `financeInKindFee` is LIVE in m1.

**`redeemReservation` (L293)** and **`retryRedeemReservation` (L469)** — redemption settlement.

> L309: `require(fee <= maxFeeTbtc, "Fee exceeds the caller's bound")` — this reverts, but only on a caller-supplied bound, not on reserve or pause state.

These are user-initiated, not Bridge-settled, so they may revert on caller errors. But once the Bridge has confirmed the Bitcoin spend, the redemption request itself is settled by the Bridge independently. The vault's role is to unmint and approve Bank balance; the only reverts are owner-check and fee-bound-check, which are caller-controlled, not settlement-gating.

### Accounting-path functions (governance/guardian bookkeeping)

`pauseRenewals`, `unpauseRenewals`, `blockRenewal`, `unblockRenewal`, `setRenewalGuardian`, `updateFeeReserveTarget`, `sweepFees`, `updateFees`, `receiveBalanceApproval` (stub), and the constructor. None of these touch a confirmed Bitcoin spend, so they are free to revert on access or state checks.

### Initiation-path functions (the only ones a pause flag may gate)

**`receiveBalanceIncrease` (L234):** this is where initiation happens. The Bridge proves an anchor, credits the Bank, the Bank calls this vault, and the vault mints gross TBTC and charges the initiation fee. Natspec L226-228: "Called by the Bank when the Bridge proves a reservation's anchor transaction and credits the gross anchored amount." The initiation fee is charged here (L254: `uint256 fee = (grossTbtc * initiationFeeBps) / BASIS_POINTS`).

In variant B, the initiation path is gated not by a vault-internal pause flag but by the Bridge's vault-trust status: deposits revealed with an untrusted vault address are not routed as reservations. The deploy script comment (L95:6-9) confirms: "Vault trust is the safe activation boundary." However, established fact 1 requires initiation disabled behind a pause flag. The vault has no `initiationPaused` flag; the existing `renewalsPaused` does NOT cover `receiveBalanceIncrease`. See Open Questions.

**`extendCustody` (L367):** renewal/initiation of a new custody term. Already gated by `renewalsPaused` (L388) and `renewalBlocked` (L389). This is the existing model for how a pause flag gates an initiation-path function.

---

## 3. Existing pause machinery (renewal pattern)

The vault has two independent pause/block mechanisms for renewals. Milestone 1 must copy this pattern for redemption.

### Global renewal pause

**Flag:** `bool public renewalsPaused` (L92).

**Natspec (L87-91):** "True while all renewals are paused. A fresh vault starts paused; governance unpauses as part of activation. Pausing only removes future renewal opportunities -- it never shortens a term already purchased and has no effect on redemption, re-anchoring or dissolution."

**Restrictive setter (guardian or owner, immediate):**
- `pauseRenewals()` L409, modifier `onlyGuardianOrOwner` (L175). Body: `renewalsPaused = true; emit ReservationRenewalsPaused(msg.sender)` (L410-411). Natspec L405-408: "Restrictive and monotonic: callable by the guardian or the owner, effective immediately, and without any effect on already-purchased terms or on redemption/re-anchor/dissolution."

**Restorative setter (owner only):**
- `unpauseRenewals()` L415, modifier `onlyOwner`. Body: `renewalsPaused = false; emit ReservationRenewalsUnpaused(msg.sender)` (L416-417). Natspec L414: "Restorative: owner (governance) only."

**Constructor default:** L221: `renewalsPaused = true`. Natspec L219-220: "A fresh vault starts with renewals paused; governance unpauses as part of the activation ceremony, after ownership has been transferred out of the deployer's hands."

**Coverage check:**
- `extendCustody` (L388): `require(!renewalsPaused, "Renewals are paused")` -- COVERED.
- `receiveBalanceIncrease`: NO pause check -- NOT covered.
- `redeemReservation`: NO pause check -- NOT covered.
- `retryRedeemReservation`: NO pause check -- NOT covered.
- `financeInKindFee`: NO pause check -- NOT covered (by design; settlement must not revert).
- All other functions: no pause check -- NOT covered.

### Per-reservation renewal block

**Flag:** `mapping(uint256 => bool) public renewalBlocked` (L98).

**Restrictive setter:** `blockRenewal(uint256 reservationKey)` L424, `onlyGuardianOrOwner`. Body: `renewalBlocked[reservationKey] = true; emit ReservationRenewalBlocked(reservationKey)` (L425-426).

**Restorative setter:** `unblockRenewal(uint256 reservationKey)` L432, `onlyOwner`. Body: `renewalBlocked[reservationKey] = false; emit ReservationRenewalUnblocked(reservationKey)` (L433-434).

**Coverage check:**
- `extendCustody` (L389): `require(!renewalBlocked[reservationKey], "Reservation renewal blocked")` -- COVERED.
- All other functions: not covered.

### Guardian role

**Storage:** `address public renewalGuardian` (L106).

**Natspec (L100-105):** "Address allowed to apply the restrictive policy actions (pause renewals, block a reservation) besides the owner. Guardian actions are monotonic -- they cannot disturb an already-paid term or move funds -- so they are safe to make immediate. Only the owner (governance) can relax policy: unpause, unblock, or replace the guardian."

**Modifier:** `onlyGuardianOrOwner` (L175-181): `require(msg.sender == renewalGuardian || msg.sender == owner(), "Caller is not the renewal guardian or owner")`.

**Setter:** `setRenewalGuardian(address newGuardian)` L440, `onlyOwner`. Emits before setting (L441-444). Zero address is deliberate (L442-443: "leaves policy actions to the owner alone").

### Pattern summary for implementing redemption pause in m1

To copy this pattern for redemption, the m1 rewrite would add:
1. `bool public redemptionsPaused` storage field, defaulting to `true` in the constructor.
2. `pauseRedemptions()` external `onlyGuardianOrOwner` that sets `redemptionsPaused = true` and emits.
3. `unpauseRedemptions()` external `onlyOwner` that sets `redemptionsPaused = false` and emits.
4. A `require(!redemptionsPaused, ...)` guard at the top of `redeemReservation` and `retryRedeemReservation`.
5. The guard must NOT appear in `financeInKindFee` (settlement must not revert).
6. The guard must NOT appear in `receiveBalanceIncrease` unless the initiation pause is a separate flag (see Open Questions).

The pattern is: restrictive actions (pause, block) are immediate and guardian-or-owner callable; restorative actions (unpause, unblock) are owner-only; the flag defaults to paused; settlement is never gated.

---

## 4. In-kind fee economy

|Item|Source|PR|Kind|m1|m2|Note|
|---|---|---|---|---|---|---|
|`feeReserveTarget` storage L115|ReservationVault.sol:115|?|storage|yes|yes|TBTC amount the vault retains as in-kind fee reserve; governable via `updateFeeReserveTarget`|
|`inKindFeeDebtSat` storage L122|ReservationVault.sol:122|?|storage|yes|yes|Outstanding in-kind fee debt in satoshi; live in m1 because re-anchor produces it|
|`initiationFeeBps` storage L77|ReservationVault.sol:77|?|storage|flagged|yes|Initiation fee in bps; charged in `receiveBalanceIncrease` (initiation-path); m1 charges it if initiation is enabled, otherwise dormant|
|`extensionFeeBps` storage L80|ReservationVault.sol:80|?|storage|flagged|yes|Extension fee in bps; charged in `extendCustody`; gated by `renewalsPaused`, so dormant when paused|
|`redemptionFeeBps` storage L85|ReservationVault.sol:85|?|storage|yes|yes|Redemption fee in bps; charged in `redeemReservation`; live if redemption is not paused (DECISION NEEDED on m1 redemption pause)|
|`updateFeeReserveTarget` L599 `onlyOwner`|ReservationVault.sol:599|?|entry-point|yes|yes|Sets the reserve target; accounting-path; required activation step|
|`financeInKindFee` L529 (Bridge-only)|ReservationVault.sol:529|?|entry-point|yes|yes|Burns TBTC from reserve to cover miner fee; records shortfall as debt; LIVE in m1 because re-anchor is a m1 essential|
|`repayInKindFeeDebt` L568 (permissionless)|ReservationVault.sol:568|?|entry-point|yes|yes|Anyone repays in-kind fee debt by burning TBTC; settlement-path adjunct; permissionless|
|`sweepFees` L613 `onlyOwner`|ReservationVault.sol:613|?|entry-point|yes|yes|Owner sweeps fee revenue above `feeReserveTarget` to treasury; accounting-path|
|`updateFees` L629 `onlyOwner`|ReservationVault.sol:629|?|entry-point|yes|yes|Governance updates all three fee parameters; accounting-path|
|Fee reserve accrual in `receiveBalanceIncrease` L250-272|ReservationVault.sol:250|?|internal|flagged|yes|Initiation fee stays in vault as reserve (L271-272 comment); dormant if initiation paused|
|Fee reserve accrual in `redeemReservation` L308|ReservationVault.sol:308|?|internal|yes|yes|Redemption fee stays in vault as reserve (L311-312 comment)|
|Fee reserve accrual in `extendCustody` L384|ReservationVault.sol:384|?|internal|flagged|yes|Extension fee stays in vault as reserve (L389-390 comment); dormant if renewals paused|
|Re-anchor fee financing (Bridge call) L874|ReservationProofs.sol:874|?|internal|yes|yes|Bridge calls `financeInKindFee(minerFee)` during re-anchor settlement; LIVE in m1|
|Dissolution fee financing (Bridge call) L995|ReservationProofs.sol:995|?|internal|no|yes|Bridge calls `financeInKindFee(dissolutionFee)` during dissolution settlement; m2 only (dissolution omitted in m1)|

### Who may call each fee function

- **`financeInKindFee`:** Bridge only (L530). Called during re-anchor (ReservationProofs.sol:874) and dissolution (ReservationProofs.sol:995) settlement.
- **`repayInKindFeeDebt`:** anyone (L568, no access modifier). Permissionless debt repayment.
- **`updateFeeReserveTarget`:** vault owner/governance only (L599, `onlyOwner`).
- **`sweepFees`:** vault owner/governance only (L613, `onlyOwner`).
- **`updateFees`:** vault owner/governance only (L629, `onlyOwner`).

### What is LIVE in m1 vs dormant

Under variant B, re-anchor charges an in-kind miner fee. Therefore:
- **LIVE in m1:** `financeInKindFee` (re-anchor path), `inKindFeeDebtSat` (debt accrual), `repayInKindFeeDebt` (debt repayment), `feeReserveTarget` (reserve target), `updateFeeReserveTarget` (setter), `sweepFees` (sweep), `updateFees` (fee schedule).
- **Dormant in m1 if initiation is disabled:** `initiationFeeBps` accrual (no anchors being proved means no `receiveBalanceIncrease` calls), `extensionFeeBps` accrual (renewals paused).
- **Dissolution fee financing is m2 only:** the dissolution path (ReservationProofs.sol:995) is omitted in m1.

---

## 5. Upgradeability posture

### Verdict: NOT proxy-upgradeable. The vault is a plain immutable contract.

**Contract declaration (L57):**
```solidity
contract ReservationVault is IVault, IReservationFeeFinancer, Ownable {
```
No `Upgradeable` base, no initializer, no proxy pattern. It inherits `Ownable` (OpenZeppelin), not `OwnableUpgradeable` or `UUPSUpgradeable`.

**Constructor (L183-222):**
```solidity
constructor(
    Bank _bank,
    TBTCVault _tbtcVault,
    IReservationBridge _bridge
) {
    require(address(_bank) != address(0), "Bank can not be the zero address");
    require(address(_tbtcVault) != address(0), "TBTCVault can not be the zero address");
    require(address(_bridge) != address(0), "Bridge can not be the zero address");

    bank = _bank;
    tbtcVault = _tbtcVault;
    tbtcToken = _tbtcVault.tbtcToken();
    bridge = _bridge;
    ...
    renewalsPaused = true;
}
```
A real constructor (not an `initialize` function), immutable references (`bank`, `tbtcVault`, `tbtcToken`, `bridge` are `immutable` at L69-72), and no proxy admin. Once deployed, the bytecode cannot be replaced.

### Re-point path and exact gate conditions

To replace the vault, governance must re-point the Bridge's `reservationVault` address. This is done via `BridgeGovernance.finalizeReservationParametersUpdate()` (BridgeGovernance.sol:1900-1910), which calls `IReservationBridge(address(bridge)).updateReservationParameters(...)` with a new `reservationVault` address.

The gate is in `Reservation.updateReservationParameters` (Reservation.sol:1263-1271):
```solidity
if (reservationVault != self.reservationVault) {
    require(
        self.reservationTotalAmount == 0,
        "Active reservations exist"
    );
    require(
        self.pendingReservedDeposits == 0,
        "Pending reserved deposits exist"
    );
    self.reservationVault = reservationVault;
    emit ReservationVaultUpdated(reservationVault);
}
```

**Exact gate conditions for re-pointing:**
1. `reservationVault != self.reservationVault` (the address must actually change) (L1263).
2. `self.reservationTotalAmount == 0` (zero active reservations) (L1264-1266).
3. `self.pendingReservedDeposits == 0` (zero pending reserved deposits) (L1267-1270).

**Implication for milestone 1:** In variant B, positions close only when their custodying wallet is terminated (**stranding**, not dissolution — `notifyReservationStranded` requires `WalletState.Terminated` at `Reservation.sol:1374-1378`, while `requestReservationDissolution` has no such gate), and stranding never drives `reservationTotalAmount` to zero in normal operation. Therefore `reservationTotalAmount` cannot reach zero through normal lifecycle during m1, which means the vault address cannot be re-pointed while the product is in use. **This is the gate condition the 2026-09-24 Option B decision weighs against shipping every entry point behind a pause flag**: since the vault can never be re-pointed while quiescence is unreachable, the alternative M1 actually took is to ship the minimal vault as-is and defer redemption/renewal to a *new* vault deployment plus depositor migration ceremony in M2, rather than pre-shipping their entry points now behind flags that could never be safely swapped out later either.

---

## 6. Deployment and activation

### What the deploy script does (`95_deploy_reservation_vault.ts`)

1. **Fetches existing deployment addresses** for `Bank`, `TBTCVault`, and `Bridge` (L8-10).
2. **Deploys `ReservationVault`** with constructor args `[Bank.address, TBTCVault.address, Bridge.address]` (L12-17). `waitConfirmations: 1`.
3. **Verifies on Etherscan** if the network has the `etherscan` tag (L26-28).
4. **Tags:** `["ReservationVault"]`. **Dependencies:** `["Bank", "TBTCVault", "Bridge"]` (L34) — Bridge IS a listed deploy dependency (fixed by `48bc9f03 fix(deploy): declare Bridge as dependency of ReservationVault`; see `pr-map.md` §2).

Ownership transfer is a separate script, not one of `95`'s own steps:

### `96_transfer_reservation_vault_ownership.ts`

Transfers ownership from `deployer` to `governance` via `helpers.ownable.transferOwnership("ReservationVault", governance, deployer)` (L8-12). `func.dependencies = ["ReservationVault"]`, `func.runAtTheEnd = true`.

### `97_set_reservation_parameters.ts`

Wires the vault into the Bridge and marks it trusted. Its own docblock (L11-41) states the activation sequence explicitly, **caps before parameters**:
1. `beginReservationCapsUpdate(...)` then `finalizeReservationCapsUpdate()` — **MUST run first**. The script's own comment explains why: `reservationMaxTotalAmount` defaults to `0`, so the relational check the parameters step depends on (in `Reservation.updateReservationParameters`) passes trivially against a still-zero cap; running parameters first would fail that relational check.
2. `beginReservationParametersUpdate(...)` then `finalizeReservationParametersUpdate()` — wires the vault address into the Bridge (the `reservationVault` arg is the update's first parameter; there is no separate `setReservationVault` setter) and sets the rest of the reservation parameters.
3. `setVaultStatus(vault, true)` via `BridgeGovernance`, as the **final** step — marks the vault as trusted. Until this runs, deposits cannot be revealed with the vault.

On local/test networks where the timelock is bypassed, begin/finalize may run in the same deploy invocation; on live non-mainnet networks only `begin*` runs here and `finalize*` follows separately after the governance delay elapses. The script is skipped entirely on mainnet (`func.skip`) pending timelock review; mainnet activation instead runs through `98_generate_reservation_mainnet_calldata.ts` plus a manual runbook.

### `98_generate_reservation_mainnet_calldata.ts`

Generates the mainnet governance calldata for the same bootstrap actions (caps, then parameters, then `setVaultStatus`), validating the known Timelock/Council-Safe addresses on-chain before emitting anything. Gated by `DEPLOY_RESERVATION_BOOTSTRAP_CALDATA=true`; not run as part of the ordinary deploy pipeline.

### `06a_deploy_reservation_router.ts` (upstream ordering dependency)

Deploys `ReservationProofs`, `Reservation`, and `ReservationRouter`, then calls the Bridge's one-time `setReservationRouter`. Must run after `06_deploy_bridge.ts` and before `97_set_reservation_parameters.ts`: every reservation governance setter (`updateReservationCaps`, `updateReservationParameters`) is a router selector reached through `Bridge.fallback()`'s delegatecall, so with no router set, script 97 would fail on "Reservation router not set".

### Governance steps required to activate, in order

1. `BridgeGovernance.beginReservationCapsUpdate(...)` then `finalizeReservationCapsUpdate()` — sets per-wallet and single-amount caps. **Must run first** (see script 97 above; params-first fails the relational check against a still-zero cap).
2. `BridgeGovernance.beginReservationParametersUpdate(...)` then `finalizeReservationParametersUpdate()` — sets the reservation vault address, min amount, tx max fee, term, dissolution delay, max total amount, max reservations per wallet, action timeout, renewal window.
3. `ReservationVault.updateFeeReserveTarget(...)` — sets the in-kind fee reserve target.
4. As the FINAL step, `BridgeGovernance.setVaultStatus(vault, true)` — marks the vault as trusted in the Bridge, which is the safe activation boundary.

Steps 1-3 must be completed while the vault is untrusted.

(The old full-vault design additionally staged a renewal guardian and an `unpauseRenewals` call between steps 3 and 4; the M1 vault has neither `setRenewalGuardian` nor `unpauseRenewals` — see "M1 vault surface" above.)

---

## Full storage and events inventory

### Storage variables

|Item|Source|PR|Kind|m1|m2|Note|
|---|---|---|---|---|---|---|
|`SATOSHI_MULTIPLIER` L61 `uint256 public constant`|ReservationVault.sol:61|?|storage|yes|yes|10**10; converts satoshi to TBTC units|
|`BASIS_POINTS` L64 `uint256 public constant`|ReservationVault.sol:64|?|storage|yes|yes|10000; bps divisor|
|`MAX_FEE_BASIS_POINTS` L67 `uint256 public constant`|ReservationVault.sol:67|?|storage|yes|yes|500; upper bound per fee parameter|
|`bank` L69 `Bank public immutable`|ReservationVault.sol:69|?|storage|yes|yes|Bank contract reference|
|`tbtcVault` L70 `TBTCVault public immutable`|ReservationVault.sol:70|?|storage|yes|yes|TBTCVault contract reference|
|`tbtcToken` L71 `TBTC public immutable`|ReservationVault.sol:71|?|storage|yes|yes|TBTC token reference (derived from tbtcVault)|
|`bridge` L72 `IReservationBridge public immutable`|ReservationVault.sol:72|?|storage|yes|yes|Bridge contract reference|
|`initiationFeeBps` L77 `uint16 public`|ReservationVault.sol:77|?|storage|flagged|yes|Initiation fee; charged in receiveBalanceIncrease; dormant if initiation disabled|
|`extensionFeeBps` L80 `uint16 public`|ReservationVault.sol:80|?|storage|flagged|yes|Extension fee; charged in extendCustody; dormant when renewalsPaused|
|`redemptionFeeBps` L85 `uint16 public`|ReservationVault.sol:85|?|storage|yes|yes|Redemption fee; charged in redeemReservation|
|`renewalsPaused` L92 `bool public`|ReservationVault.sol:92|?|storage|yes|yes|Global renewal pause flag; defaults true; gates extendCustody only|
|`renewalBlocked` L98 `mapping(uint256=>bool) public`|ReservationVault.sol:98|?|storage|yes|yes|Per-reservation renewal block; gates extendCustody only|
|`renewalGuardian` L106 `address public`|ReservationVault.sol:106|?|storage|yes|yes|Guardian allowed restrictive policy actions|
|`feeReserveTarget` L115 `uint256 public`|ReservationVault.sol:115|?|storage|yes|yes|TBTC amount retained as in-kind fee reserve|
|`inKindFeeDebtSat` L122 `uint64 public`|ReservationVault.sol:122|?|storage|yes|yes|Outstanding in-kind fee debt in satoshi; live in m1 via re-anchor|

### Events

|Item|Source|PR|Kind|m1|m2|Note|
|---|---|---|---|---|---|---|
|`ReservationCreditProcessed` L124|ReservationVault.sol:124|?|event|flagged|yes|Emitted in receiveBalanceIncrease; dormant if initiation disabled|
|`CustodyExtended` L130|ReservationVault.sol:130|?|event|flagged|yes|Emitted in extendCustody; dormant when renewalsPaused|
|`ReservedRedemptionInitiated` L136|ReservationVault.sol:136|?|event|yes|yes|Emitted in redeemReservation and retryRedeemReservation|
|`FeesUpdated` L143|ReservationVault.sol:143|?|event|yes|yes|Emitted in updateFees|
|`ReservationRenewalBlocked` L149|ReservationVault.sol:149|?|event|yes|yes|Emitted in blockRenewal|
|`ReservationRenewalUnblocked` L151|ReservationVault.sol:151|?|event|yes|yes|Emitted in unblockRenewal|
|`ReservationRenewalsPaused` L153|ReservationVault.sol:153|?|event|yes|yes|Emitted in pauseRenewals|
|`ReservationRenewalsUnpaused` L155|ReservationVault.sol:155|?|event|yes|yes|Emitted in unpauseRenewals|
|`RenewalGuardianUpdated` L157|ReservationVault.sol:157|?|event|yes|yes|Emitted in setRenewalGuardian|
|`InKindFeeFinanced` L162|ReservationVault.sol:162|?|event|yes|yes|Emitted in financeInKindFee; LIVE in m1 via re-anchor|
|`InKindFeeDebtRepaid` L164|ReservationVault.sol:164|?|event|yes|yes|Emitted in repayInKindFeeDebt|
|`FeeReserveTargetUpdated` L166|ReservationVault.sol:166|?|event|yes|yes|Emitted in updateFeeReserveTarget|
|`FeesSwept` L168|ReservationVault.sol:168|?|event|yes|yes|Emitted in sweepFees|

### Modifiers

|Item|Source|PR|Kind|m1|m2|Note|
|---|---|---|---|---|---|---|
|`onlyBank` L170|ReservationVault.sol:170|?|internal|yes|yes|`require(msg.sender == address(bank))`; gates receiveBalanceIncrease|
|`onlyGuardianOrOwner` L175|ReservationVault.sol:175|?|internal|yes|yes|`require(msg.sender == renewalGuardian OR msg.sender == owner())`; gates pause/block setters|

### Invariants (unstated but implied by code)

|Item|Source|PR|Kind|m1|m2|Note|
|---|---|---|---|---|---|---|
|Gross-mint invariant|ReservationVault.sol:248-249|?|invariant|yes|yes|Total TBTC minted against a reservation always equals the sats earmarked on-chain; fee is an explicit transfer, never netted (natspec L45-47, L248-249)|
|Settlement-never-reverts invariant|ReservationVault.sol:524-528|?|invariant|yes|yes|A confirmed Bitcoin spend must never fail to settle because of the reserve level; shortfall recorded as debt (financeInKindFee natspec)|
|Pause-is-monotonic invariant|ReservationVault.sol:87-91, L100-105|?|invariant|yes|yes|Pause/block never shortens an already-paid term and never moves funds; only owner can relax|
|Re-point-quiescence invariant|Reservation.sol:1264-1270|?|invariant|yes|yes|Vault address can change only when reservationTotalAmount==0 and pendingReservedDeposits==0|
|Claim-equals-anchor invariant|ReservationProofs.sol:866-872|?|invariant|yes|yes|Re-anchor writes mintedAmount down to newAnchorAmount so the claim surrendered at redemption always equals the sats on-chain|

---

## Open questions

1. **DECISION NEEDED: How is initiation disabled in m1?** The vault has no `initiationPaused` flag. The existing `renewalsPaused` flag gates `extendCustody` only, NOT `receiveBalanceIncrease`. The deploy script says "Vault trust is the safe activation boundary" and that `unpauseRenewals` is "not a global pause for reserved deposit reveals." Three options: (a) add a new `initiationPaused` flag following the renewal pause pattern, with a `require(!initiationPaused)` guard in `receiveBalanceIncrease`; (b) rely solely on Bridge vault-trust status (`setVaultStatus`) to gate initiation, shipping no vault-internal flag; (c) overload `renewalsPaused` to also guard `receiveBalanceIncrease`. Established fact 1 says "disable INITIATION behind a pause flag," which implies option (a), but the existing code and deploy comments point to option (b). Which is intended?

2. **DECISION NEEDED: Is redemption paused in m1?** Under variant B, milestone 2 restores redemption. The existing `renewalsPaused` does NOT cover `redeemReservation` or `retryRedeemReservation`. If redemption must be disabled in m1, a new `redemptionsPaused` flag (copying the renewal pattern) is needed, with guards in both redemption entry points. But the rule also says "never gate settlement or accounting," and redemption is arguably settlement-adjacent. Is redemption considered initiation-path (pausable) or settlement-path (never pausable) for m1 purposes? The `redeemReservation` function reverts on caller errors (fee bound, ownership), so it is not a pure must-never-revert settlement function like `financeInKindFee`, but the Bitcoin spend has not yet occurred when it is called (it initiates the redemption request). Clarification needed: should m1 ship `redeemReservation` active, flagged behind a pause, or omitted?

3. **DECISION NEEDED: Is `retryRedeemReservation` needed in m1?** It only exists because a fee-paid redemption timed out through the wallet's fault. If `redeemReservation` is disabled in m1, `retryRedeemReservation` has no precursor and is dead code. But the rule says ship every entry point. Should it ship flagged (present but unreachable) or active?

4. **DECISION NEEDED: What happens to `receiveBalanceIncrease` when initiation is disabled?** If initiation is disabled, the Bridge will not prove anchors to this vault (because the vault is untrusted or because a new flag prevents processing). But `receiveBalanceIncrease` is `onlyBank` and has no pause guard. If the Bank somehow calls it (e.g., a late settlement for a deposit revealed before the pause), it will mint TBTC and charge the initiation fee. Is this acceptable, or must `receiveBalanceIncrease` also be guarded by the initiation pause flag? Note the `ReservationProofs.sol:563-567` late-settlement routing comment: the vault address at acceptance time is used, not the live `reservationVault`, so late settlements can still call `receiveBalanceIncrease` even after the vault is re-pointed or untrusted.

5. **DECISION NEEDED: PR origin attribution.** The stated PR range is `#1088`, `#1090`-`#1096`, `#1102`, but the source files contain no PR references, commit hashes, or changelog entries that would let me attribute individual items to specific PRs from the file contents alone. All PR columns are marked `?`. Is there a git history or PR-to-file mapping available that I should consult?

6. **DECISION NEEDED: Does m1 need `dissolutionFeeBps` or a dissolution fee parameter?** The vault has no dissolution fee parameter; dissolution miner fees are computed on-chain by the Bridge (ReservationProofs.sol:994: `uint64 dissolutionFee = inputsTotalValue - outputValue`) and financed via `financeInKindFee`. Since dissolution is omitted in m1, the dissolution fee-financing call path (ReservationProofs.sol:995) is dormant. But `financeInKindFee` itself must still be present and active because re-anchor uses it. Confirm: m1 ships `financeInKindFee` active, and the dissolution call path is simply not exercised because the dissolution proof submission is omitted?

7. **DECISION NEEDED: Is `sweepFees` safe to call in m1?** If initiation is disabled and no fees have accrued, the vault TBTC balance is zero and `sweepFees` will revert on `require(balance > feeReserveTarget)`. This is fine (accounting-path may revert). But if `feeReserveTarget` is set to zero and some residual TBTC exists, `sweepFees` could drain the reserve needed for re-anchor fee financing. Should m1 enforce a non-zero `feeReserveTarget` before enabling re-anchor? The deploy script lists `updateFeeReserveTarget` as a required step (step 3), implying yes.

   **Finding 2026-09-07 (new failure mode, this question stays open):**
   the real failure mode is an unconditional revert, not a drain.
   `_burnFromReserve` ignores `feeReserveTarget` entirely when repaying
   debt — it burns `min(amount, full current balance)` with no floor.
   `sweepFees`'s debt-first repayment (`:380-388`) calls it with the full
   `inKindFeeDebtSat`, so the burn attempts past the target regardless of
   how `feeReserveTarget` is configured. The *following*
   `require(balance > feeReserveTarget)` (`:391`), checked against the
   *post-burn* balance, is what stops that attempt from persisting — a
   revert here unwinds the whole call, repayment included, per normal
   Solidity semantics. Net effect: the reserve itself is never actually
   drained (a nonzero target still protects the *outcome*), but once
   `inKindFeeDebtSat` is large enough to breach it, `sweepFees` becomes
   unconditionally uncallable — not "may drain the reserve" (the original
   framing), but "always reverts, blocking ordinary fee sweeping along with
   debt repayment" until either fresh fee income grows the balance or
   someone calls `repayInKindFeeDebt` directly. Decided fix
   (`pr-review-followups.md` item 8, `roadmap.md` §7 item 6): cap debt
   repayment at `feeReserveTarget` so the burn itself never attempts past
   it. Not yet implemented.

8. **Not covered by stated rules: Does the m1 rewrite preserve the exact storage layout?** Since the vault is not upgradeable, storage layout matters only if a migration is planned (which requires total quiescence, unreachable in m1). But if m1 is a rewrite ("variant B"), the new contract has a fresh address and fresh storage. The question is whether any m1 caller (Bridge, Bank, governance) stores the vault address and whether re-pointing is blocked by active reservations. It is (see re-point gate). So the m1 rewrite deploys at a new address and cannot be re-pointed until quiescence. This is the fundamental constraint driving the ship-everything-now rule.

9. **Not covered by stated rules: Who is the `renewalGuardian` in m1?** The deploy script says appointing a guardian is optional (step 4). If no guardian is appointed, `renewalGuardian` is `address(0)` and the restrictive setters (`pauseRenewals`, `blockRenewal`) are callable only by the owner. Is that acceptable for m1, or must a guardian be appointed before activation?

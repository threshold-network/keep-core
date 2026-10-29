# Reservation hand-off capacity

This design separates admission limits from evacuation of existing anchors.
Governance retains control of the per-wallet amount and count limits.
The policy applies to the M1 reservation feature before Sepolia activation.

## Admission reserve

Let `P` be `activeReservationsCount`, including pending acceptances.
Let `L` be `liveWalletsCount`, `N` be `maxReservationsPerWallet`, and `R` be `handoffReserveWallets`.
A new acceptance must satisfy both conditions:

- `P + 1 <= maxActiveReservations`.
- `P + 1 <= max(0, L - R) * N`.

The request must also satisfy every existing amount and target-wallet cap.
The Bridge checks the current values when the depositor requests acceptance.
A reveal does not reserve capacity.
The SDK checks the same budget before funding and before requesting acceptance.
Its checks cannot prevent a later capacity race.

Governance must set `R > 0`.
The reserve gate rejects admission when `L <= R`.
For Sepolia, `L = 4`, `N = 1`, and `R = 2` give a budget of two positions.
`maxActiveReservations = 2` remains a separate hard limit.

The gate does not reserve an identified wallet or a permanently empty slot.
A re-anchor temporarily holds source and target capacity.
Further retirements, governance rotations, cap reductions and late proofs can consume the intended margin.
Already-confirmed Bitcoin spends remain settleable under their existing proof rules.
The reserve gate must not reject their settlement.

## Privileged hand-off

| Source | Caller | Target count and amount caps |
|---|---|---|
| `Live` | Governance only | Enforced |
| `MovingFunds` or `Closing` | Governance, reservation owner, SPV maintainer, or proven source-wallet member | Not enforced |
| `MovingFunds` or `Closing` | Any other caller | Enforced, together with the third-party hold cap |

Every request still adds its target count and amount.
The source retains capacity until settlement.
A timeout releases the target capacity.
A successful hand-off does not create a new position or increase the global position count.
New acceptances cannot use an already over-cap wallet.

Privileged hand-offs can concentrate anchors above the normal per-wallet limits.
This is the explicit tradeoff that removes cap-induced retirement deadlocks.
keep-core prefers a Live wallet with ordinary headroom.
If none has headroom, a source-member request selects the least-loaded eligible Live wallet.
A pending third-party hold cannot block this separate privileged request merely by filling the target's ordinary capacity.

The exception does not bypass other authorization requirements.
The source must be retiring and the target must be a different Live wallet.
The reservation must be Active and its anchor must satisfy the amount floor.
Only the governance-discriminated call bypasses the re-anchor cooldown.
A caller cannot replace an already-pending action on the same reservation.

The proposal validator checks the authorized action, wallet states, signing window, source anchor and fee rules.
It does not recheck target capacity that the request already booked.
This rule applies to both privileged and third-party authorizations.

## Governance and ABI

`maxReservationsPerWallet` remains in the delayed reservation-parameters update.
`maxReservationsAmountPerWallet` remains in the delayed reservation-caps update.
Zero disables only the amount cap.
The caps setter and `beginReservationCapsUpdate` now accept five arguments:

```solidity
updateReservationCaps(
    uint64 maxReservationsAmountPerWallet,
    uint64 reservationMaxSingleAmount,
    uint32 maxActiveReservations,
    uint32 maxThirdPartyReanchorHoldsPerWallet,
    uint32 handoffReserveWallets
)
```

`ReservationCapsUpdated` contains the same five fields.
`reservationCaps()` remains a four-value view.
`reservationHandoffReserveWallets()` exposes the reserve separately.
Clients, governance calldata, event decoders and generated bindings must use the new ABI.

If governance raises the normal per-wallet count, it should review the amount limit too.
The emergency procedure uses the existing begin/finalize delay.
No faster privileged setter is added.

## Limit of the safety claim

The design removes per-wallet-cap rejection as a blocker for otherwise valid privileged evacuation.
It does not guarantee a surviving Live target, an available signing quorum, Bitcoin confirmation or proof delivery.
A single reserved slot alone cannot give that guarantee because hand-offs and retirements consume capacity over time.
Monitor wallet availability, action deadlines and concentration as well as the global position count.

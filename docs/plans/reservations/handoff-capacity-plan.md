# Hand-off capacity (D31: design C + A) - change plan

Decision (human, 2026-10-12): build C + A before the Sepolia deploy. The existing per-wallet setters stay as they are. The runbook gets an emergency step: raise `maxReservationsPerWallet` (and the per-wallet amount cap) when free target slots run low.

## Design

**C. Uncapped privileged hand-off.** In `requestReservationReanchor`, when the source wallet is `MovingFunds` or `Closing` and the caller is privileged (governance, owner, SPV maintainer, or proven source member), skip the target's per-wallet count and amount cap checks. The counts and amounts are still added. Third-party requests keep both caps and the hold cap. A governance rotation off a Live source keeps both caps.

**A. Hand-off reserve gate at acceptance.** New governance value `handoffReserveWallets` (uint32, must be at least 1). `reserveAcceptanceCapacity` requires:
`liveWalletsCount > handoffReserveWallets` and `activeReservationsCount + 1 <= (liveWalletsCount - handoffReserveWallets) * maxReservationsPerWallet`, else revert "Hand-off reserve exceeded".

The gate limits admission using the current Live wallet count. It does not reserve an identified wallet or slot. Pending rotations, later retirements, late proofs and cap changes can consume the intended margin. C removes per-wallet count and amount caps as blockers for otherwise valid privileged hand-offs. It does not guarantee Bitcoin signing, proof delivery, an available Live wallet, or an immediate retry during cooldown.

## Surfaces and changes

| # | Surface | Repo | Change | Red test (must fail before the fix) |
|---|---|---|---|---|
| 1 | `Reservation.requestReservationReanchor` | tbtc-v2 | C | A privileged request from a `MovingFunds` source to a full target succeeds. Third-party to a full target still reverts. Governance from a Live source to a full target still reverts |
| 2 | `Reservation.reserveAcceptanceCapacity` | tbtc-v2 | A gate | An acceptance at `(L - R) * N` reverts "Hand-off reserve exceeded" while `maxActiveReservations` still has room. The gate follows a falling `liveWalletsCount` |
| 3 | `Reservation.updateReservationCaps` | tbtc-v2 | New fifth argument `handoffReserveWallets`, must be at least 1. Event `ReservationCapsUpdated` gains the field | 0 reverts |
| 4 | `BridgeState.Storage` | tbtc-v2 | New `uint32 handoffReserveWallets`, packed with no layout shift (storage-parity tests) | Covered by the parity tests |
| 5 | `IReservationBridge`, `ReservationRouter` | tbtc-v2 | Setter selector changes. New view `reservationHandoffReserveWallets()`. `reservationCaps()` stays a 4-tuple | - |
| 6 | `BridgeGovernance` + `BridgeGovernanceParameters` | tbtc-v2 | `beginReservationCapsUpdate` takes 5 arguments; the pending value is stored and applied at finalize | Finalize applies the reserve |
| 7 | `WalletProposalValidator` re-anchor checks | tbtc-v2 | Drop the target per-wallet count and amount checks. The request already reserved the capacity, and C lets privileged actions exceed the caps. All other checks stay | A proposal whose target is above the cap now validates |
| 8 | Scripts: 97, 98, Sepolia rehearsal, fork dry-run, `RESERVATION_*` docs, runbook `cast` lines | tbtc-v2 | 5-argument caps call. Sepolia `handoffReserveWallets = 2` (4 Live wallets, N = 1, so 2 positions; matches max active 2). Mainnet fork value set by its env | Script runs |
| 9 | All tbtc-v2 tests and executors that call the caps setter | tbtc-v2 | Updated for the new arity | Full reservation suites pass |
| 10 | keep-core re-anchor target choice (`walletHasReanchorHeadroom`, target search) | keep-core | For a member-proof request from a `MovingFunds`/`Closing` source, prefer targets with headroom and fall back to any Live target other than the source. The resume path and third-party behaviour do not change | With every Live target at the cap, the task now requests a re-anchor; before, it found no target |
| 11 | keep-core bindings, vendored ABI drift gate, `StubBridge` + validator harness bytecode | keep-core | Regenerate from the new tbtc-v2 artifacts | Harness: the real validator accepts an above-cap target |
| 12 | Spec (`docs/reservations-spec-reanchor-cap`) | keep-core | FR for C and A; D19 residual risk closed | - |
| 13 | Drill runbook, handoff, Sepolia runbook, guide | both | D4: owner request to a held target now succeeds. Residual risk R1 removed. Capacity schedule uses the gate | - |
| 14 | Fork rehearsal (`reservations-fork-dryrun`) | tbtc-v2 | New steps on the mainnet fork (see below) | - |
| 15 | SDK reservation pre-funding check and acceptance preflight | tbtc-v2 | Read the reserve view and Live wallet count. Report no-go when the next acceptance would exceed `(L - R) * N`, including a caller-selected wallet | A deposit with ordinary cap headroom but no hand-off headroom is refused before funding |

## Fork rehearsal steps (after 1-9 land)

Mainnet fork at the pinned block, with `liveWalletsCount` read from the fork. Set caps so that the gate binds before `maxActiveReservations`.
1. Accept positions up to `(L - R) * N`. The next acceptance reverts "Hand-off reserve exceeded".
2. Put the source wallet of one accepted position into `MovingFunds`, through the real Bridge path if one is reachable on the fork. Otherwise use one recorded storage write, logged as seeded state like the D18 relay pointer.
3. Fill a Live target to the cap. Then:
   - a privileged (owner) re-anchor to that target succeeds;
   - a third-party re-anchor to the same full target reverts "Wallet reservations cap exceeded";
   - a third-party re-anchor to a free target succeeds.
4. `WalletProposalValidator.validateReservationReanchorProposal` (`eth_call`) passes for the above-cap privileged action.

## Order

1. tbtc-v2 contracts, tests, scripts (surfaces 1-9), branch a handoff-capacity branch from the integrated reservation branch. keep-core logic (surface 10), branch a handoff-capacity branch from the reservation review branch. These two run in parallel.
2. keep-core ABI and harness sync (11). Fork rehearsal (14). Merge into the Sepolia rehearsal branch and re-run the Sepolia rehearsal.
3. Spec and docs (12, 13).

## Integration constraints

- Preserve the completed release-to-pool and SDK features in the integrated reservation branch at `df481bfde`.
- The orchestrator re-ran regressions against immutable baseline `9d2086965`: 9 behavioral failures and 3 unchanged controls. The patched run passed all 12.
- The two gate tests now target a wallet with available per-wallet capacity. Their baseline failure is an accepted transaction, not an unrelated cap revert.
- The client fallback red check already ran against the actual pre-fix Go production file and passed after the fix.
- The SDK uses the new reserve gate too. Keep its pre-funding and acceptance preflight aligned with the contract.

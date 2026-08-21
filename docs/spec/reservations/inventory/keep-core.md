# keep-core wallet-side inventory (PR #4238)

Source-verified on `origin/feat/utxo-reservation-wallet-support`.
Diffstat against `origin/main`: 11 files, +1833 -9.

| File | Change |
|---|---|
| `pkg/tbtc/reservation_test.go` | +770 |
| `pkg/tbtc/reservation.go` | +729 |
| `pkg/chain/ethereum/tbtc.go` | +93 |
| `pkg/tbtc/chain.go` | +52 |
| `pkg/clientinfo/performance_test.go` | +51 |
| `pkg/tbtc/chain_test.go` | +49 |
| `pkg/tbtc/wallet_test.go` | +46 -? |
| `pkg/tbtc/wallet.go` | +28 |
| `pkg/tbtc/marshaling.go` | +16 -? |
| `pkg/clientinfo/performance.go` | +4 |
| `pkg/tbtcpg/redemptions.go` | +4 -? |

## 1. Correction: `#4238` is not "the single-phase design"

Every current doc describes `#4238` as implementing the **original single-phase
design**, needing rework for "nonce-carrying proposals, a
watchtower-delay-respecting executor, and partial-redemption awareness"
(`feature-spec.md` §16, `roadmap.md` §5, `epic-merge-plan.md` §3). Measured
against the source, that characterisation is wrong in one direction and
understated in the other.

**Nonces are already there.** `RequestNonce` appears 14 times and is a field on
**all four** proposal structs:

| Proposal | Line | Fields |
|---|---|---|
| `ReservationAnchorProposal` | `reservation.go:181` | `DepositFundingTxHash`, `DepositFundingOutputIndex`, **`RequestNonce`**, `AnchorTxFee` |
| `ReservedRedemptionProposal` | `:233` | `ReservationKey`, **`RequestNonce`**, `RedemptionTxFee` |
| `ReservationReanchorProposal` | `:287` | `ReservationKey`, **`RequestNonce`**, `TargetWalletPublicKeyHash`, `ReanchorTxFee` |
| `ReservationDissolutionProposal` | `:342` | `ReservationKey`, **`RequestNonce`**, `DissolutionTxFee` |

And the chain interface reads action records by generation:
`GetReservationAction(reservationKey, requestNonce)` (`chain.go:437-440`). Both
are two-phase constructs. A purely single-phase client would have neither.

**The real gap is larger and different: there is no client at all.** See §3.

## 2. What `#4238` does have

| Item | Source | Kind | m1 | m2 | Note |
|---|---|---|---|---|---|
| `ReservationState` enum | `reservation.go:30-54` | type | yes | yes | Mirrors Solidity `ReservationState` |
| `ReservationActionType` enum | `:86-96` | type | yes | yes | Mirrors `ActionType` |
| `ReservationActionState` enum | `:98-110` | type | yes | yes | Mirrors `ActionState` |
| `Reservation` struct | `:56-84` | type | yes | yes | The position record |
| `ReservationAction` struct | `:112-145` | type | yes | yes | The action record |
| `ReservationParameters` struct | `:147-179` | type | yes | yes | Governance parameter mirror |
| `ReservationAnchorProposal` + 4 methods | `:181-231` | type | yes | yes | `ActionType`, `ValidityBlocks`, `Marshal`, `Unmarshal` |
| `ReservationReanchorProposal` + 4 methods | `:287-340` | type | yes | yes | m1's unpin path |
| `ReservedRedemptionProposal` + 4 methods | `:233-285` | type | **declare only** | yes | m1 never proposes one |
| `ReservationDissolutionProposal` + 4 methods | `:342-396` | type | **declare only** | yes | Variant B's cut |
| `assembleReservationAnchorTransaction` | `:398` | internal | yes | yes | Bitcoin tx assembly |
| `assembleReservationReanchorTransaction` | `:581` | internal | yes | yes | |
| `computeReservationRedeemerOutputScriptHash` | `:557` | internal | yes | yes | Shared helper |
| `assembleReservedRedemptionTransaction` | `:449` | internal | no | yes | |
| `assembleReservationDissolutionTransaction` | `:628` | internal | no | yes | |
| `GetReservation` | `chain.go:432` | interface | yes | yes | Bound at `ethereum/tbtc.go` |
| `GetReservationAction` | `chain.go:437-440` | interface | yes | yes | Two-phase read |
| `ReservationParameters` | `chain.go:444` | interface | yes | yes | |
| `ValidateReservationAnchorProposal` | `chain.go:449` | interface | yes | yes | Bound |
| `ValidateReservationReanchorProposal` | `chain.go:469` | interface | yes | yes | Bound |
| `ValidateReservedRedemptionProposal` | `chain.go` | interface | no | yes | Bound; m2 only |
| `ValidateReservationDissolutionProposal` | `chain.go:477` | interface | no | yes | Bound; m2 only |
| `ActionReservationAnchor` (enum 6) | `wallet.go` | type | yes | yes | Plus string and metrics names |
| `ActionReservationReanchor` (enum 8) | `wallet.go` | type | yes | yes | |
| `ActionReservedRedemption` (enum 7) | `wallet.go` | type | **declare only** | yes | Positional: cannot be renumbered |
| `ActionReservationDissolution` (enum 9) | `wallet.go` | type | **declare only** | yes | Positional |

`reservation.go` has **21 functions**: 19 touch m1-relevant paths (mostly the
four proposals' `Marshal`/`Unmarshal`/`ActionType`/`ValidityBlocks` sets, which
m1 needs for at least two of the four), and 2 are m2-exclusive assemblers.

**Note on the action-type enum.** `ActionReservedRedemption` is 7 and
`ActionReservationDissolution` is 9, decoded positionally from the wire
(`case 7:`, `case 9:`). Like the Solidity enums, these positions cannot be
compacted in m1 without silently reinterpreting persisted values.

## 3. What `#4238` lacks: the entire executor

This is the finding that matters for sizing m1.

**No proposal generators.** `pkg/tbtcpg/` is the proposal-generation package.
It contains a task per wallet action - `NewDepositSweepTask`,
`NewRedemptionTask`, `NewHeartbeatTask`, `NewMovingFundsTask`,
`NewMovedFundsSweepTask` (`tbtcpg.go`). There is **no reservation task**, and
grepping the whole package case-insensitively for `reservation` returns **zero
files**. The only change `#4238` makes there is 4 lines in `redemptions.go`.

Consequence: nothing ever *decides* to propose a reservation action. The
proposal types exist and can be marshalled, but no code produces one.

**No submission methods.** The chain interface's reservation region
(`chain.go:430-480`) contains only reads and validators. There is no
`Submit*`, `Request*` or `Notify*` method. The Ethereum binding
(`pkg/chain/ethereum/tbtc.go`) implements exactly six reservation methods, all
read or validate:

```
GetReservation
GetReservationAction
ReservationParameters
ValidateReservationAnchorProposal
ValidateReservationReanchorProposal
ValidateReservationDissolutionProposal
```

Consequence: the client cannot submit an acceptance request, cannot submit an
SPV proof, and cannot notify an action timeout. Every write path is absent.

**So `#4238` is a types-and-assembly foundation, not a client.** It can read
chain state, validate a proposal it is handed, and build a Bitcoin transaction.
It cannot participate in the protocol.

## 4. m1 keep-core work, restated

Because the executor is absent rather than misshapen, m1's Go work is mostly
**new code, not rework**. That is a different risk profile: less untangling,
more greenfield, and no reviewed baseline to inherit.

| Work item | Nature | Note |
|---|---|---|
| Reservation proposal-generator task in `pkg/tbtcpg` | **New** | Must mirror the existing task shape; acceptance and re-anchor only under B |
| Chain-interface write methods (request, submit proof, notify timeout) | **New** | Plus their Ethereum bindings |
| Re-anchor executor triggered on `WalletMovingFunds` | **New** | `m1-b-implementation.md` §5; the only unpin, so failure is not a delay |
| Below-dust report after the last re-anchor | **New** | `roadmap.md` §0.8; nothing else triggers wallet closing |
| Stranding watcher on `Terminated` wallets | **New** | |
| Stale reserved-deposit cleanup | **New** | |
| Action-timeout watch | **New** | Expiry slashes |
| Regenerated ABI bindings | Mechanical | Follows the Solidity surface, so it must come after it is stable |
| Types, enums, proposals, assemblers | **Reusable from `#4238`** | The genuine salvage |
| Redemption and dissolution proposal generation and execution | **Not m1** | B's saving, roughly 300-500 production Go |

Existing test coverage: `reservation_test.go` (+770) and `chain_test.go` (+49)
cover the types, marshalling and validators - which is precisely the part that
survives. **None of it covers an executor, because there is no executor.**

## Open questions

1. **DECISION NEEDED: does m1 keep all four proposal types in Go, or only the
   two it uses?** The Solidity enums must keep their positions, and
   `WalletActionType` is decoded positionally from the wire, so the two
   m2-only action-type constants must stay. The proposal *structs* are a
   separate question: they are not persisted on-chain, so dropping
   `ReservedRedemptionProposal` and `ReservationDissolutionProposal` in m1 is
   safe if the enum constants remain. Recommend keeping them - they are already
   written and tested, and deleting then restoring them is churn.
2. **DECISION NEEDED: is `#4238` edited, superseded, or closed?** Given the
   executor is absent rather than wrong, most of `#4238` is directly reusable
   and the m1 client is largely additive. That argues for editing or building
   on it rather than closing it. This contradicts the framing in
   `pr-strategy.md` §8, which treats it as superseded design; the sizing there
   should be revisited against this finding.
3. **UNVERIFIED: the 1,400-1,900 production Go estimate** in
   `roadmap.md` §5.1. It was derived assuming rework of a single-phase client.
   Since the work is instead a new executor plus new write plumbing, the
   estimate should be rebuilt bottom-up from the task list in §4 above.

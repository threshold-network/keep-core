# M1 UTXO Reservations - System Model (SM)

Scope: the reservation subsystem as it is pinned and shipped. Pinned commits:
`tbtc-v2` at `9d2086965` (branch `reservations-upgrade`), `keep-core`
at `f99ef96e3` (branch `reservations-epic`). Every normative claim
cites `repo:path:line`. M1 is variant B (create-only, minimal vault): no
redemption, renewal, or dissolution entry points exist; the vault is
`Ownable`, not upgradeable. The release-to-pool rows (SM-D5, the depositor
row in SM-A1, and the SM-V1 release guard) use the later tbtc-v2 pin
`d20cf74ba`; the D27 SDK component rows (SM-C11, SM-C12, SM-C13, SM-C14) use
the tbtc-v2 pin `7aabfc502` (branch `reservations-upgrade`, with
SDK commits `3d114b15e` and `7768a3370`); the post-release keep-core sweep
behaviour (SM-A8, SM-D5 keep-core half, SM-C8) uses the later keep-core pin
`4b95f0a06` (release binding `8a714f2c1` plus the post-release sweep tests
`4b95f0a06`); the D31 hand-off rows (SM-C15, and the D31 clauses in SM-A2
to SM-A5, SM-A7, SM-R1 and SM-R5) use the tbtc-v2 pin `806b1106f` (branch
`reservations-upgrade`) and the keep-core pin `d40f6aadc` (branch
`reservations-epic`) and always carry an explicit `@` revision; every
other citation in this file is at `9d2086965` / `f99ef96e3`.

# Recorded decisions (M1, do not reopen)

- **Variant B (Option B minimal vault):** M1 is create-only: acceptance,
  custody, re-anchor, stranding. No redemption, renewal, or dissolution
  entry points. The vault is `Ownable`, not upgradeable; M2 needs a new
  vault plus migration. `ReservationProofs.sol:168` accepts only Acceptance
  and Reanchor proofs; `submitReservationProof` reverts on the other
  `ProofType` values.
- **D19 (third-party re-anchor holds):** Re-anchor from MovingFunds/Closing
  sources is permissionless. A non-privileged request reserves the target's
  ordinary slot and is capped per target by
  `maxThirdPartyReanchorHoldsPerWallet` (`Reservation.sol:866`); the owner,
  SPV maintainers, proven source-wallet members, and governance are exempt
  (`Reservation.sol:786`). At `9d2086965` that a third-party hold still
  consumes the target's ordinary slot, and that a privileged hand-off to a
  cap-full target reverted, was a known, accepted risk. D31 closes the
  privileged half at `tbtc-v2@806b1106f` (see D31 below and SM-C15): a
  third-party hold still consumes the target's ordinary slot for
  third-party requests, but it no longer blocks a privileged hand-off from a
  retiring source.
- **D26 (release to pool):** Implemented at `tbtc-v2@d20cf74ba`
  (commit `638c48663`): the depositor may release a revealed, never-requested
  reserved deposit back to the ordinary pool
  (`releaseReservedDepositToPool`, `tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1373`). See SM-D5.
- **D27 (SDK reservation module):** Implemented in `tbtc-v2@7aabfc502`
  (SDK commits `3d114b15e` reserved-deposit support, `7768a3370` dust and
  global-count check for a chosen wallet). The consumer-facing SDK exposes
  pre-funding go/no-go, wallet selection, reveal, acceptance request,
  release-to-pool, and status reads (SM-C11, SM-C12, SM-C13). The on-chain
  behaviour it wraps is at the pins already cited in this file. Never
  describe the SDK as an on-chain change: it is a thin read/write client
  over the reservation router ABI fragments, and a go/no-go or preflight
  "yes" does not guarantee the on-chain call is accepted (TOCTOU, SM-C14).
- **D31 (hand-off capacity, C + A):** Approved as a pre-Sepolia change. It
  is NOT in the original pins (`9d2086965`, `7aabfc502`, `4b95f0a06`). It is
  implemented and tested on the reservations branches at tbtc-v2 `806b1106f` and
  keep-core `d40f6aadc`; nothing is deployed on Sepolia or mainnet. Design C:
  privileged re-anchors off a `MovingFunds`/`Closing` source (governance,
  owner, SPV maintainer, proven source member) skip the target's per-wallet
  count/amount cap checks (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`);
  third-party requests keep both caps and the hold cap, and a governance
  rotation off a `Live` source keeps both caps. Design A: a new
  `handoffReserveWallets` (uint32, >= 1) gate at acceptance
  (`reserveAcceptanceCapacity`, `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1052-1077`),
  a new fifth caps-setter argument and a new view
  (`reservationHandoffReserveWallets`,
  `tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:519`).
  Closes the D19 residual (a privileged handoff to a cap-full target). The
  D27 SDK pre-funding and acceptance-preflight checks consume the reserve
  view at `tbtc-v2@806b1106f:typescript/src/services/reservations/reservations-service.ts:707-722`.
  Status `holds` at the new pins (SM-C15); the same behavior is absent at
  the original pins. Cross-reference: SM-C11..SM-C14, SM-C15.
- **D28 (maintainer keepers + alerts):** Implemented at keep-core `9c4e1b221`
  and tbtc-v2 `8eb26c2e2`. An independent existing `maintainer` instance
  publishes four risk gauges and an observation timestamp. The existing
  monitoring service sends Critical and Warning events to Sentry.
  Historical wallet discovery includes registrations before activation.
  Local acceptance is recorded in [approved design deltas](13-approved-design-deltas.md).
  Live Sepolia rollout remains pending. Cross-reference: SM-C16.

Status vocabulary: `holds` (code enforces it and a test pins it),
`holds-untested` (code enforces it, no test found by reading), `violated`
(code breaks it: exact path given), `design-only` (planned, not in code),
`unknown` (say what is missing).

IDs: SM- (this file). Other files own PAR-, INV-, US-, EC-, FM-, DR-, TM-,
XR-, AR- prefixes; cross-references by topic are allowed, invented IDs are
not.

## Actors and roles

| ID | Actor | What it can call (M1 surface) | Code citation | Status |
|---|---|---|---|---|
| SM-A1 | Depositor (the `deposit.depositor` address that revealed a reserved deposit) | `revealDeposit` routing to the reservation vault (via `Bridge.revealDeposit` -> `Deposit._revealDeposit`), then `requestReservationAcceptance` with the funding transaction, its SPV proof, the output index and the wallet (only the depositor may call it; `msg.sender == deposit.depositor`; the router derives the reservation key from the proven funding transaction hash and the output index, `tbtc-v2@c8350d109:solidity/contracts/bridge/ReservationRouter.sol:230-247`). At the later pin `d20cf74ba`, the depositor may also call `releaseReservedDepositToPool` on a never-requested reserved deposit (SM-D5). | `tbtc-v2@c8350d109:solidity/contracts/bridge/Deposit.sol:388` (sets `pendingReservedDeposit[key]`) | holds |
| SM-A2 | Reservation owner (in M1 this is the depositor, set at acceptance settlement: `reservation.owner = deposit.depositor`) | Privileged re-anchor exemption: `requestReservationReanchor` treats `reservation.owner == msg.sender` as privileged and exempts it from the third-party hold cap. At `tbtc-v2@806b1106f` a privileged owner request off a MovingFunds/Closing source also skips the target count and amount caps (SM-C15; test `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:427`). | `Reservation.sol:788` | holds |
| SM-A3 | Wallet members / operators (source-wallet signing-group members) | Privileged re-anchor via a non-empty member proof: `requestReservationReanchor(..., walletMembersIDs, walletMemberIndex)`; a non-empty proof that fails validation reverts. Members are exempt from the third-party hold cap. At `tbtc-v2@806b1106f` a proven member request off a MovingFunds/Closing source also skips the target count and amount caps (SM-C15; test `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:452`). | `Reservation.sol:774` (member proof), `:790` (exempt) | holds |
| SM-A4 | SPV maintainer (approved on-chain via `Bridge.setSpvMaintainerStatus`) | `submitReservationAcceptanceProof`, `submitReservationReanchorProof` (router `onlySpvMaintainer` gate); privileged re-anchor exemption (`isSpvMaintainer[msg.sender]`). At `tbtc-v2@806b1106f` a maintainer request off a MovingFunds/Closing source also skips the target count and amount caps (SM-C15; test `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:443`). | `ReservationRouter.sol:268`, `:294`; `Reservation.sol:789` | holds |
| SM-A5 | Governance (the `BridgeGovernance` contract, owner via `onlyOwner`; the `governance` address is a contract, never an EOA) | `updateReservationParameters`, `updateReservationCaps` (both `onlyGovernance` on the router, staged through `BridgeGovernance` begin/finalize), the staged caps slot being readable through `pendingReservationCapsUpdate()` (`tbtc-v2@c8350d109:solidity/contracts/bridge/BridgeGovernance.sol:1999-2019`); `forceStaleReservedDeposit` (governance only, immediate: `BridgeGovernance.forceStaleReservedDeposit` is `onlyOwner` with no governance delay, `tbtc-v2@c8350d109:solidity/contracts/bridge/BridgeGovernance.sol:1939-1941`); privileged re-anchor (caller `msg.sender == governance`, `ReservationRouter.requestReservationReanchor` passes `privileged = (msg.sender == governance)`), reachable only through `BridgeGovernance.requestReservationReanchor(uint256,bytes20)`, `onlyOwner` and not staged behind a governance delay, which forwards to the reservation surface at the Bridge address with empty `walletMembersIDs` and member index 0 and is the only caller the router admits for a `Live` source (`tbtc-v2@c8350d109:solidity/contracts/bridge/BridgeGovernance.sol:1973-1983`). Both entry points come from tbtc-v2#1179 and are present at the freeze pin. At `9d2086965` `updateReservationCaps` takes four arguments (selector `0xf858967d`); at `tbtc-v2@806b1106f` it takes five (adds `handoffReserveWallets`, selector `0x78d76eda`, router entry `tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:476`, staging `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1896`) and rejects a zero reserve ("Hand-off reserve must be greater than zero" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1701`; test `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationCaps.test.ts:163`). A governance rotation off a `Live` source keeps both target caps (test `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:485`). | `ReservationRouter.sol:377`, `:455`, `:410`, `:248`; `BridgeGovernance.sol:1834`, `:1893`, `:1936` (all at `9d2086965`), `:1973` and `:1999` at `tbtc-v2@c8350d109` | holds |
| SM-A6 | Vault owner (the `Ownable` owner of `ReservationVault`, governance) | `updateInitiationFee`, `updateFeeReserveTarget`, `sweepFees` (all `onlyOwner` on the vault). The vault is `Ownable`, not upgradeable. | `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:278`, `:302`, `:335` | holds |
| SM-A7 | Third party (any address not owner/maintainer/governance/member) | Permissionless `requestReservationReanchor` against a MovingFunds/Closing source; capacity is reserved on the target wallet and bounded by `maxThirdPartyReanchorHoldsPerWallet`. `notifyReservationActionTimeout`, `notifyReservationAcceptanceTimedOut`, `notifyStaleReservedDeposit`, `notifyReservationStranded` are all permissionless. At `tbtc-v2@806b1106f` a third-party request still keeps both target caps and the hold cap (tests `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:466` count, `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:502` amount). | `Reservation.sol:866` (hold cap), `:934`, `:667`, `:1229`, `:1192` | holds |
| SM-A8 | keep-core node (orchestrator/tbtcpg maintainer + SPV maintainer) | Proposes and signs anchor/re-anchor transactions via `WalletProposalValidator.validateReservationAnchorProposal` / `validateReservationReanchorProposal`; submits SPV proofs; runs timeout, stale-deposit, stranding, and proof watchers. At `keep-core@f99ef96e3` the ordinary deposit sweep read a released-to-pool deposit's vault from the reveal event (`keep-core:pkg/tbtcpg/deposit_sweep.go:252`), which was correct but had no dedicated keep-core test (`holds-untested`); at `keep-core@4b95f0a06` the post-release sweep is pinned by behavioural tests (see the keep-core sweep note below). See XR- section. | `WalletProposalValidator.sol:963`, `:1108`; `keep-core:pkg/maintainer/spv/reservation_wiring.go:301`; `keep-core:pkg/tbtcpg/deposit_sweep.go:252` | holds |

Note: `closeReservation` (`Reservation.sol:1141`) and the redemption/dissolution
action types are m2-only; they have no M1 caller. `ActionState.Vetoed`
(`Reservation.sol:138`) and the watchtower-delay fields
(`Reservation.sol:306-312`) are declared for m2 storage stability; no M1 code
path ever writes `Vetoed` or the watchtower fields. Marked as unreachable in
M1 (below).

## Components and trust boundaries

| ID | Component | Role | Trust boundary / note |
|---|---|---|---|
| SM-C1 | Bridge proxy | Canonical state owner; falls back to the router for any unmatched selector. | All reservation state lives in `BridgeState.Storage`; the router executes there via `delegatecall`. `tbtc-v2:solidity/contracts/bridge/Bridge.sol:2119` (fallback). |
| SM-C2 | ReservationRouter (`delegatecall`) | External reservation ABI and shared storage layout. | The router declares its own `BridgeState.Storage self`, but calls through the Bridge execute against Bridge storage. The acceptance entry point proves the deposit's funding transaction before it delegates (`tbtc-v2@c8350d109:solidity/contracts/bridge/ReservationRouter.sol:230-247`), so reveal alone cannot reserve capacity. Standalone router storage is not the canonical ledger. Storage parity and selector disjointness are separate obligations (`ReservationRouter.sol:103`, `Bridge.sol:2119`). |
| SM-C3 | Reservation library | Request/authorization side: acceptance, re-anchor, timeout, stranding, stale-deposit, release-to-pool (at `d20cf74ba`), governance setters. | `Reservation.sol:60`. |
| SM-C4 | ReservationProofs library | SPV settlement side: acceptance and re-anchor proofs; late settlement; strand-on-settlement. | `ReservationProofs.sol:65`. |
| SM-C5 | ReservationVault | Liability side of a reservation; mints TBTC gross, charges an initiation fee, finances in-kind miner fees. | `Ownable`, not upgradeable. `ReservationVault.sol:46`. |
| SM-C6 | WalletProposalValidator | Validates proposals and signing windows; it does not produce Bitcoin signatures. | Operators sign after validation. Contract view entry points are `WalletProposalValidator.sol:963` and `:1108`. |
| SM-C7 | Bank / TBTC | `Bank.increaseBalanceAndCall` routes the gross anchor credit to the (trusted) vault; `bank.increaseBalances` is the direct-credit fallback when the vault is untrusted. | `Bank.sol:337`, `:302`. |
| SM-C8 | keep-core tasks (ReservationAcceptanceTask, ReservationReanchorTask) | Client-side proposal generation for anchor and re-anchor within the coordination checklist, gated on the network activation block. | `keep-core:pkg/tbtcpg/reservation_acceptance.go:93`, `keep-core:pkg/tbtcpg/reservation_reanchor.go:76`. |
| SM-C9 | SPV maintainer (proof loop, watchers) | Submits proofs, reports timeouts, clears stale deposits, and strands. | `keep-core:pkg/maintainer/spv/reservation_proof_loop.go:355`, `reservation_wiring.go:301`. |
| SM-C10 | C-4 activation gate | keep-core only starts coordinating reservations at the network's `reservationsActivationBlocks` block B; `setVaultStatus(vault, true)` must land at or after B. | `keep-core:pkg/tbtc/coordination.go:110`. |
| SM-C11 | SDK reservation handle (`Reservations` / `EthereumReservations`) | Consumer-facing read/write client over the reservation router ABI fragments, bound at the Bridge address. Exposes `requestReservationAcceptance`, `releaseReservedDepositToPool`, and the view surface (SM-C13); merges `reservationBridgeAbiFragments` into the Bridge deployment artifact so the reservation surface is reached through the Bridge fallback. | `tbtc-v2@7aabfc502:typescript/src/lib/ethereum/reservations.ts:148` (`withReservationFragments`), `:165` (`EthereumReservations`), `:196` (`requestReservationAcceptance`), `:212` (`releaseReservedDepositToPool`); fragments at `tbtc-v2@7aabfc502:typescript/src/lib/ethereum/artifacts/reservation-bridge-fragments.ts:127`, release fragment `tbtc-v2@7aabfc502:typescript/src/lib/ethereum/artifacts/reservation-bridge-fragments.ts:140`, release event `tbtc-v2@7aabfc502:typescript/src/lib/ethereum/artifacts/reservation-bridge-fragments.ts:315`. Status `holds`. |
| SM-C12 | SDK reservation service (`ReservationsService`) | Pre-funding go/no-go (`checkReserveFeasibility`), deterministic wallet selection (`selectReservationWallet`), reserved-deposit initiation (`initiateReservedDeposit`), reveal (`revealReservedDeposit`), acceptance request (`requestAcceptance`) and its preflight (`checkAcceptancePreflight`), release-to-pool (`releaseToPool`), and status reads. Reads live on-chain parameters/caps/counters to make go/no-go and preflight decisions; it does not write any new state - every mutation is a router call. | `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:140` (`ReservationsService`), `:167` (`checkReserveFeasibility`), `:283` (`selectReservationWallet`), `:343` (`initiateReservedDeposit`), `:419` (`revealReservedDeposit`), `:456` (`requestAcceptance`), `:478` (`checkAcceptancePreflight`), `:574` (`releaseToPool`), `:590` (`reservationRecord`). Status `holds`. |
| SM-C13 | SDK status reads (view surface) | The read-only views the service consumes: `reservations`, `reservationActions`, `reservationParameters`, `reservationCaps`, `activeReservationsCount` (returns a `(count, maxActive)` tuple, `tbtc-v2@7aabfc502:typescript/src/lib/ethereum/reservations.ts:333`), `walletReservationsAmount`/`walletReservationsCount`, `pendingReservedDeposits`, `reservedDepositWallet`, `pendingThirdPartyReanchorHolds`, `isReservedDeposit`, `deposit`, `wallet`, `isVaultTrusted`, `depositParameters`, plus `NewWalletRegistered` and `ReservedDepositReleasedToPool` event fetchers. The owner address is surfaced as `reservationRecord.owner` (zero reads as `undefined`, `tbtc-v2@7aabfc502:typescript/src/lib/ethereum/reservations.ts:236`). | `tbtc-v2@7aabfc502:typescript/src/lib/ethereum/reservations.ts:228` (`reservations`), `:256` (`reservationActions`), `:287` (`reservationParameters`), `:313` (`reservationCaps`), `:333` (`activeReservationsCount`), `:420` (`isReservedDeposit`), `:430` (`deposit`), `:459` (`wallet`); interface at `tbtc-v2@7aabfc502:typescript/src/lib/contracts/reservations.ts:269`. Status `holds`. |
| SM-C14 | SDK pre-funding TOCTOU boundary (limit) | The go/no-go and preflight checks read live on-chain state and then the on-chain request re-checks it in a separate transaction. A `go` result (or a preflight `ok`) is an off-chain prediction, not an acceptance guarantee: capacity, wallet state, dust, caps, the owner/depositor match, and the signing window can all change between the read and the request. The SDK also cannot evaluate the deadline-based signing-window rules off-chain (the deposit's snapshotted refund deadline is not a public view), so `checkAcceptancePreflight` reports all other rules but skips those. | `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:467` (`checkAcceptancePreflight` doc: "The deadline-based signing-window rules are not checked here"), `:505` (owner/depositor match), `:526` (wallet Live check, re-run at request time); on-chain re-checks at `Reservation.sol:512` (depositor), `tbtc-v2@9d2086965:solidity/contracts/bridge/Reservation.sol:554` (Live), `tbtc-v2:solidity/contracts/bridge/Reservation.sol:562` (amount), `tbtc-v2@9d2086965:solidity/contracts/bridge/Reservation.sol:586` (signing window). Status `holds-untested` (the SDK check is unit-tested at `typescript/test/services/reservations.test.ts`; the guarantee it does NOT provide is the on-chain re-check, pinned by US-04/US-05, not by any SDK assertion). |
| SM-C15 | D31 hand-off admission (implemented at `tbtc-v2@806b1106f` and `keep-core@d40f6aadc`; not deployed) | Rule 1 (hand-off): a privileged re-anchor off a MovingFunds/Closing source skips the target's per-wallet count and amount caps; bookkeeping still adds the target count and amount, and the source keeps its capacity until settlement. Rule 2 (reserve gate): a new acceptance request must satisfy P + 1 <= max(0, L - R) * N, where P is `activeReservationsCount` (includes pending acceptances), L is `liveWalletsCount`, R is `handoffReserveWallets` and N is `maxReservationsPerWallet`. Rule 3: the caps setter takes five arguments and rejects R = 0. The SDK preflight and the client target search apply the same rules. | Code at `tbtc-v2@806b1106f`: `skipTargetCaps = isPrivileged && sourceState != Live` at `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`; privileged is the governance flag, the owner, an SPV maintainer or a non-empty member proof (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:814-818`); count cap skipped at `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:928-933`, amount cap skipped at `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:942-948`, target count and amount added on every path (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:934-935`, `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:949-950`). Gate: "Hand-off reserve exceeded" `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1076`, computed from `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1071` and called once from `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:643`. Storage `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeState.sol:509`, view `tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:519`; `reservationCaps()` stays a four-value view (`tbtc-v2@806b1106f:solidity/contracts/bridge/ReservationRouter.sol:495`). Tests at `tbtc-v2@806b1106f`: gate `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:322` (reverts past (L - R) * N with max-active room), `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:346` (a Live wallet leaving Live tightens admission), `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:379` (no acceptance while L <= R); hand-off describe `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:359` (owner, SPV maintainer, member and amount-cap cases succeed for a privileged caller), third party and Live-source governance still revert (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:466`, `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:485`), invalid member proof reverts (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:235`). Signing validator: at `9d2086965` it re-checked the target count cap ("Wallet reservations cap exceeded" `tbtc-v2:solidity/contracts/bridge/WalletProposalValidator.sol:1171`); at `806b1106f` it enforces the target Live check only (doc block `tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1092-1098`) and the real validator harness accepts an above-cap target (`keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1205`, `keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1219`). SDK: `tbtc-v2@806b1106f:typescript/src/services/reservations/reservations-service.ts:255-262` (feasibility reason), `tbtc-v2@806b1106f:typescript/src/services/reservations/reservations-service.ts:567-574` (acceptance preflight), `tbtc-v2@806b1106f:typescript/src/services/reservations/reservations-service.ts:707-722` (it returns false when N is 0, while the chain would reject every acceptance), read handle `tbtc-v2@806b1106f:typescript/src/lib/ethereum/reservations.ts:333`; tests `tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:414` (describe) and `tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:542` (preflight). Client: member proof gathered at `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:262-290`, least-loaded fallback target `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor.go:1195-1241`, tests `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor_test.go:1479` (member falls back) and `keep-core@d40f6aadc:pkg/tbtcpg/reservation_reanchor_test.go:1611` (non-member still sends nothing when full). Limits: the gate is a count-admission bound checked at the acceptance request only (not at proof time, re-anchor or settlement); it does not guarantee a Live target, a signing quorum, proof delivery, amount headroom or fee room; hand-offs above the cap concentrate anchors on one wallet; a Live-source rotation stays governance-only and keeps both caps; R reads 0 on an upgraded Bridge until the first five-argument caps update, and the setter enforces R >= 1 only when it runs. Original pins: no `handoffReserveWallets`, a four-argument setter, and both target caps checked for every caller (INV-203, INV-204 at `9d2086965`). Nothing is deployed on Sepolia or mainnet. Evidence summary: [approved deltas](13-approved-design-deltas.md). Status: holds. |
| SM-C16 | D28 maintainer keepers + alerts (implemented; live rollout pending) | Existing `maintainer` role observes late-proof risk, unusable re-anchor targets, vault debt and deposit request deadlines. The monitor also detects failed or stale observations. | Status `holds` for implementation and recorded local acceptance, not deployment. Gauges: `keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:346-373`; alerts: `tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:157-225`. Tests and per-stage acceptance: [approved design deltas](13-approved-design-deltas.md), INV-115, US-100, EC-23 and EC-24. |

## State machines

### ReservationState (position state)

Enum at `Reservation.sol:74`. In M1 the reachable values are `Unknown`,
`Active`, `ActionPending`, `Closed` (declared but never written in M1), and
`Stranded`. `Closed` is only produced by `closeReservation`, which has no M1
caller, so it is unreachable in M1.

| ID | From | Trigger (entry point) | Caller | Guards (condensed) | To | Counters / storage written | Test | Status (intended / observed) |
|---|---|---|---|---|---|---|---|---|
| SM-R1 | Unknown | `requestReservationAcceptance` | depositor | vault set; deposit revealed, unswept, reserved, routed to reservationVault; wallet Live; `deposit.amount > minAmount + 2 * txMaxFee`, strict (`tbtc-v2@c8350d109:solidity/contracts/bridge/Reservation.sol:597-599`; the earlier pin `9d2086965` required only `>= minAmount + txMaxFee`, and the strict form is what PAR-X1 states); caps free (at `tbtc-v2@806b1106f` also the hand-off reserve gate P + 1 <= max(0, L - R) * N, `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1071-1077`, SM-C15); signing window non-empty; refund margin respected | Unknown (stays Unknown; pending acceptance tracked only in the action record) | `reserveAcceptanceCapacity` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1005`): `activeReservationsCount+=1`, `reservationTotalAmount+=amount`, `walletReservationInfo[w].count/amount += ...`; action record written (Acceptance, Pending). `Reservation.sol:621`, `:630` | `tbtc-v2:solidity/test/bridge/Reservation.test.ts` ("step 20: state mutations and event emission") | intended `Unknown->Unknown(+Pending action)`; observed `holds` |
| SM-R2 | Unknown | `notifyReservationAcceptanceTimedOut` | anyone | current generation is Acceptance/Pending and `now >= timeoutAt` | Unknown | action -> `TimedOut`; `releaseAcceptanceCapacity` (decrements the wallet's count and amount, the global total amount and the active count). `tbtc-v2@c8350d109:solidity/contracts/bridge/Reservation.sol:692`, `:700` | `tbtc-v2@c8350d109:solidity/test/bridge/Reservation.test.ts` acceptance-timeout group | intended `Pending->TimedOut` (position stays Unknown); observed `holds` |
| SM-R3 | Unknown | `submitReservationAcceptanceProof` (on-time) | SPV maintainer | settleable Acceptance, position Unknown, anchor single output pays target wallet, fee <= snapshotted bound, anchor >= min | Active | action -> `Settled`; position written; `reservationsByAnchorUtxo` index set; if `late=false` releases the miner-fee delta (`reservationTotalAmount-=feeDelta`, `walletReservationInfo[w].amount-=feeDelta`). `ReservationProofs.sol:543`, `:550` | `ReservationProofs.test.ts` "5. End-to-end happy path" | intended `Unknown->Active`; observed `holds` |
| SM-R4 | Unknown | `submitReservationAcceptanceProof` (late) | SPV maintainer | a TimedOut Acceptance within `termSeconds` of its timeout; re-take capacity without re-checking caps | Active (or Stranded immediately, SM-R8) | re-take `reservationTotalAmount`, `walletReservationInfo[w].count/amount`, `activeReservationsCount+=1`; unwind any newer pending generation. `ReservationProofs.sol:524`, `:506` | `ReservationProofs.test.ts` "8. Late-settle and pending newer generation unwinding" | intended `TimedOut->Active`; observed `holds` |
| SM-R5 | Active | `requestReservationReanchor` | privileged or third party | position Active; source wallet MovingFunds/Closing (or Live only for governance); target Live and != source; target has capacity (at `9d2086965` every caller; at `tbtc-v2@806b1106f` a privileged request off a MovingFunds/Closing source skips the target count and amount caps, `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`, SM-C15; third-party and Live-source requests keep them); (non-privileged: target third-party holds under cap) | ActionPending | `reservation.state=ActionPending`; `requestNonce+=1`; target count/amount reserved on every path; third-party hold +1 if counted. `Reservation.sol:906`, `:909` | `Bridge.ReservationSourceAnchorBinding.test.ts`; `Bridge.ReservationThirdPartyReanchorHolds.test.ts` (at `806b1106f`: `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:359`) | intended `Active->ActionPending`; observed `holds` |
| SM-R6 | Active | `notifyReservationStranded` | anyone | position Active; wallet Terminated or Closed, or Live with `now >= dissolutionEligibleAt`, or Closing with `now >= dissolutionEligibleAt` AND `now >= closingStartedAt + walletClosingPeriod` (`tbtc-v2@c8350d109:solidity/contracts/bridge/Reservation.sol:1325-1340`); MovingFunds is not enough | Stranded | `strandReservation`: `releaseAcceptanceCapacity` (decrements three counters), `removeAnchoredCustody` (drops the anchored-custody entry, which is what lets the retiring wallet finalize), delete `reservationsByAnchorUtxo`, `state=Stranded`. `Reservation.sol:1222-1235` at `tbtc-v2@c8350d109` | `Bridge.ReservationStranding.test.ts` "notifyReservationStranded"; `tbtc-v2@c8350d109:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:186` (Closing rejected), `:222` (Closing period not elapsed), `:263` and `:313` (both conditions met, inclusive at the end), `:386` and `:397` (Live before and after eligibility) | intended `Active->Stranded`; observed `holds` at `tbtc-v2@c8350d109` for the corrected guard. The guard recorded before VF-10 admitted neither the Live path nor the closing period. The published code and the intended policy agree: a position on a healthy (Live) wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was taken on 2026-10-29 and the code does not change. |
| SM-R7 | ActionPending (on-time), Active or qualifying Stranded (late) | `submitReservationReanchorProof` | SPV maintainer | Pending/TimedOut Reanchor, matching source outpoint and permitted position state; authorized target/output and fee; late reconstruction if necessary | Active, or Stranded on a Closed or Terminated target (a Closing target keeps the position Active, `tbtc-v2@c8350d109:solidity/contracts/bridge/ReservationProofs.sol:234-253`) | Source count/amount released, target fee delta released; global amount decreases by miner fee; global occupancy does not change for an ordinary move; custody/index updated (`ReservationProofs.sol:262-304`, `:640-821`) | `ReservationProofs.test.ts` end-to-end SPV cases | holds for the mapped transitions; no complete state-product test claimed |
| SM-R8 | Active / ActionPending / Stranded | strand-on-settlement (`strandLateSettlementIfTargetWalletClosed`) | SPV maintainer (via settlement) | the target/custody wallet is Closed or Terminated after an acceptance or re-anchor settles; a Closing target does not strand, and keeps the owner's re-anchor right (`tbtc-v2@c8350d109:solidity/contracts/bridge/ReservationProofs.sol:234-253`) | Stranded | same `strandReservation` accounting. `ReservationProofs.sol:228` | `ReservationProofs.test.ts` "9. Strand flow on late settlement"; `tbtc-v2@c8350d109:solidity/test/bridge/ReservationProofs.test.ts:1434` (Closing target stays Active), `:1308` (Closed target strands on an on-time proof) | intended `->Stranded` on settlement for Closed and Terminated targets; observed `holds` at `tbtc-v2@c8350d109`. The earlier row also listed Closing (VF-10). The published code and the intended policy agree: a position on a healthy (Live) wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was taken on 2026-10-29 and the code does not change. |
| SM-R9 | Active or ActionPending | late settlement of an older Reanchor while a newer is pending | SPV maintainer | older Reanchor TimedOut; new anchor consumed; unwind the pending newer generation | Active | unwind: newer action -> `Superseded`, release its target capacity + third-party hold. `ReservationProofs.sol:832` | `ReservationProofs.test.ts` "8. ... unwinding" | intended older `TimedOut->Active`, newer `Pending->Superseded`; observed `holds` |
| SM-R10 | Active / ActionPending | `closeReservation` | none in M1 | Internal helper's position-state guard; no M1 entry point reaches it | Closed | Release tracking and write Closed (`Reservation.sol:1141-1165`) | none in M1 | design-only; unreachable in M1 |
| SM-R11 | ActionPending | `notifyReservationActionTimeout` | anyone | Current Pending Reanchor and `now >= timeoutAt` | Active | Release target capacity and third-party hold; set TimedOut and cooldown; retain source slot and global occupancy (`Reservation.sol:934-993`) | `Bridge.ReservationStrandingLibrary.test.ts` timeout-capacity cases | holds |

Unreachable in M1: `Closed` (no `closeReservation` caller), `ActionState.Vetoed`
(no watchtower code path in M1), and `retryCredit`/`feePaid`/`usedRetryCredit`/
`isPartial`/`watchtower*`/`redeemer`/`actionDataHash`/`retryCreditSourceNonce`
fields on the action record (declared, written only as zero, never set true in
M1). `reanchorCooldownUntil` IS written in M1 (set on re-anchor timeout; read
to gate every re-anchor request except the governance call).
`Reservation.sol:241`, `:760`, `:984`.

| ID | From | Trigger | To | Notes / Test | Status (intended / observed) |
|---|---|---|---|---|---|
| SM-X1 | Unknown | request (acceptance / re-anchor) | Pending | writes a fresh `ReservationAction` record keyed by `(key, ++requestNonce)`. `Reservation.sol:631`, `:915` | intended `->Pending`; observed `holds` |
| SM-X2 | Pending | on-time proof | Settled | `ReservationProofs.sol:489`, `:734` | intended `Pending->Settled`; observed `holds` |
| SM-X3 | Pending | timeout notify | TimedOut | acceptance: `Reservation.sol:692`; re-anchor: `Reservation.sol:963` (also reverts `reservation.state` to Active + sets cooldown) | intended `Pending->TimedOut`; observed `holds` |
| SM-X4 | Pending | supersede-unwind (late settle of an older generation consuming this anchor) | Superseded | `ReservationProofs.sol:849` | intended `Pending->Superseded`; observed `holds` |
| SM-X5 | TimedOut | late proof | Settled (late) | re-takes capacity without re-checking caps; only Acceptance is window-bounded (`now <= timeoutAt + termSeconds`), Reanchor is unbounded. `ReservationProofs.sol:204`, `:200` | intended `TimedOut->Settled(late)`; observed `holds` (Acceptance window pinned; Reanchor late happy path `holds-untested`) |
| SM-X6 | TimedOut | (no path) | - | late settlement closes the anchor lineage; no second refund. `ReservationProofs.sol:57` | descriptive (no transition; describes the close of the lineage); observed `holds` |
| SM-X7 | Vetoed | (none in M1) | - | `Reservation.sol:138` | intended M2 watchtower write; observed `design-only` (no M1 code ever writes `Vetoed`) |

### Pending reserved deposit lifecycle

The `PendingReservedDeposit` struct is at `BridgeState.sol:36`; the counter is
`pendingReservedDeposits` (`BridgeState.sol:431`). At `9d2086965` `isReserved`
is set at reveal and never cleared; the wallet/refund fields are cleared on
acceptance or staleness. At the later pin `d20cf74ba` the depositor's
release-to-pool path (SM-D5) additionally clears `isReserved` itself, so a
released deposit is no longer seen as reserved by ordinary sweeps.

| ID | From | Trigger | To | Storage written | Test | Status (intended / observed) |
|---|---|---|---|---|---|---|
| SM-D1 | (not reserved) | `revealDeposit` routed to the reservation vault | reserved/pending | `pendingReservedDeposit[key]` set, `pendingReservedDeposits+=1`. `Deposit.sol:388` | `Reservation.test.ts` guard 5 | intended set pending reserved; observed `holds` |
| SM-D2 | reserved/pending | acceptance settles (on-time or late) | consumed (marker cleared) | `consumeAcceptedDeposit`: clears `walletPubKeyHash`, `refundDeadline`, `refundDeadlineValidated`; `pendingReservedDeposits-=1`; sets `deposit.sweptAt`. `ReservationProofs.sol:404` | `ReservationProofs.test.ts` "12. Idempotency guard for consumeAcceptedDeposit" | intended clear marker on settle; observed `holds` |
| SM-D3 | reserved/pending | `notifyStaleReservedDeposit` (permissionless, after `refundDeadline` elapsed and no acceptance pending) | stale (marker cleared, `isReserved` retained at `9d2086965`) | clears the three fields; `pendingReservedDeposits-=1`. `Reservation.sol:1261` | `Bridge.ReservationStranding.test.ts` "notifyStaleReservedDeposit" | intended clear marker, retain `isReserved` at `9d2086965`; observed `holds` |
| SM-D4 | reserved/pending | `forceStaleReservedDeposit` (governance; no refund-deadline check) | stale | same clears; `pendingReservedDeposits-=1`. `Reservation.sol:1309` | `Bridge.ReservationStrandingLibrary.test.ts` | intended clear marker (no deadline check); observed `holds-untested` (no dedicated on-chain test; library guards shared with SM-D3) |
| SM-D5 | reserved/pending (never requested: `reservations[key].requestNonce == 0`) | `releaseReservedDepositToPool` (depositor-only; at `d20cf74ba`) | released / ordinary deposit | Clear all four fields: `pendingDeposit.isReserved=false`, `walletPubKeyHash=0`, `refundDeadline=0`, `refundDeadlineValidated=false`; `pendingReservedDeposits-=1`; `deposit.vault = vault` (re-route to the chosen ordinary vault or zero). `tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1441`-`:1446` | `tbtc-v2@d20cf74ba:solidity/test/bridge/Bridge.ReservationReleaseToPool.test.ts` "happy path" and "releaseReservedDepositToPool guards" groups | intended release to ordinary pool (clears `isReserved`); observed `holds` at `d20cf74ba` |

SM-D5 guards (`tbtc-v2@d20cf74ba:Reservation.sol:1373`): deposit revealed and
not swept; caller is the depositor; the record is a pending reserved deposit
(`isReserved` and a non-zero designated wallet, so a stale-marked entry is
rejected); `reservations[depositKey].requestNonce == 0` (no acceptance/re-anchor
was ever requested - once a wallet may have signed an anchor for a generation,
a late proof can still settle it and a pool sweep of the already-spent input is
impossible, `:1405`); the snapshotted refund deadline still leaves the 24h
`DEPOSIT_REFUND_SAFETY_MARGIN` open in the future so an ordinary sweep can be
proposed now (`:1419`); the target is the zero address or a trusted vault that
is NOT the reservation vault itself (`:1430`, the pooled-sweep-into-reservation-
vault would let `ReservationVault.receiveBalanceIncrease` mint TBTC for any
credit). Status `holds` (a 670-line dedicated suite pins it).

### Wallet states as they affect reservations

Wallet enum at `Wallets.sol:33` (`Wallets.WalletState`: Unknown/Live/MovingFunds/Closing/Closed/Terminated). Reservation-relevant transitions:

| ID | From | Trigger | To | Effect on reservations | Test | Status (intended / observed) |
|---|---|---|---|---|---|---|
| SM-W1 | Live | `notifyWalletCloseable` / `notifyWalletHeartbeatFailed` / `notifyWalletRedemptionTimeout` (all route to `moveFunds`), only if `mainUtxoHash != 0` OR `walletAnchoredInfo[w].count != 0` | MovingFunds | A wallet that still custodies **proven** anchors is routed through MovingFunds, never straight to Closing (`tbtc-v2@c8350d109:solidity/contracts/bridge/Wallets.sol:623-641`, gate at `:629-631`; balance check with the same ledger `:350-356`). Capacity reserved for a pending acceptance or re-anchor target does not keep a wallet in MovingFunds (VF-11) | `tbtc-v2:solidity/test/bridge/Bridge.ReservationStranding.test.ts` "driven to Terminated by a real moving-funds-timeout call chain" | intended keep a wallet with proven anchors in MovingFunds; observed `holds` at `tbtc-v2@c8350d109` for the proven-anchor ledger, `tbtc-v2@c8350d109:solidity/test/bridge/Reservation.test.ts:2105` |
| SM-W2 | Live | `moveFunds` when `mainUtxoHash == 0` AND `walletAnchoredInfo[w].count == 0` | Closing (via `beginWalletClosing`) | No proven anchor and no main UTXO: immediate closing, even with capacity reserved for a pending acceptance or re-anchor (`tbtc-v2@c8350d109:solidity/contracts/bridge/Wallets.sol:629-640`; `tbtc-v2@c8350d109:solidity/test/bridge/Reservation.test.ts:2155`) | `tbtc-v2:solidity/test/bridge/Bridge.Wallets.test.ts` "should change wallet's state to Closing" | intended close immediately when nothing custodied; observed `holds` |
| SM-W3 | Closing | `notifyWalletClosingPeriodElapsed` | Closed | Hard-requires `walletReservationInfo.count == 0` before finalizing closure (`tbtc-v2@c8350d109:solidity/contracts/bridge/Wallets.sol:392-395`). This is the reserved-capacity ledger, so a pending-only reservation still blocks `Closing->Closed` even though it no longer blocks the Live `-> Closing` step (SM-W1) | `none` (the count gate itself has no dedicated Solidity test; it is exercised indirectly by `Bridge.ReservationOccupancy.test.ts`) | intended block closure while reservations custodied; observed `holds-untested` (count gate has no dedicated test) |
| SM-W4 | any | `notifyWalletMovingFundsTimeout` / fraud challenge defeat | Terminated | `terminateWallet` decrements `liveWalletsCount` when the wallet was Live/MovingFunds. `Wallets.sol:471`, `:690` | `tbtc-v2:solidity/test/bridge/Bridge.ReservationStranding.test.ts` (terminates a real wallet mid-reservation) | intended terminate and release slot; observed `holds` |
| SM-W5 | Active-reservation on a Closing/Closed/Terminated/Live wallet | `notifyReservationStranded` (permissionless) | Stranded | Live past `dissolutionEligibleAt`; Closing past `dissolutionEligibleAt` AND past `closingStartedAt + walletClosingPeriod`; Terminated and Closed always (`tbtc-v2@c8350d109:solidity/contracts/bridge/Reservation.sol:1325-1340`) | `tbtc-v2:solidity/test/bridge/Bridge.ReservationStranding.test.ts` "notifyReservationStranded" | intended strand an active position on a retiring wallet; observed `holds` (Terminated, Closed, dissolution-eligible Live and Closing-past-both pinned; see US-13, US-14 and US-101 for the per-wallet-state detail). The published code and the intended policy agree: a position on a healthy (Live) wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was taken on 2026-10-29 and the code does not change. |

Key invariant: a wallet cannot finalize closing (MovingFunds -> Closing ->
Closed) while it still custodies reservation anchors. The check is enforced
at `moveFunds`'s routing decision and at `notifyWalletClosingPeriodElapsed`;
it is deliberately absent from `beginWalletClosing`/`finalizeWalletClosing`
(invoked after a moving-funds BTC transaction is already proven).
`Wallets.sol:596-609`, `:384`. Status `holds`.

### Vault trust status

| ID | Element | Behavior | Code citation | Status |
|---|---|---|---|---|
| SM-V1 | `isVaultTrusted[vault]` | Toggled by `Bridge.setVaultStatus` (governance only); read by acceptance settlement and - at `d20cf74ba` - by the release-to-pool target check. | `BridgeState.sol:289`; `Bridge.sol:1288`; `ReservationProofs.sol:621` (settlement branch); `tbtc-v2@d20cf74ba:Reservation.sol:1430` (release guard: target must be the zero address or a trusted vault that is not the reservation vault) | holds (on-time path pinned in `ReservationProofs.test.ts` "11. Vault-routing regression test"; the untrusted-credit branch and the release trust guard are `holds-untested` at `9d2086965` - the release guard is `holds` at `d20cf74ba` via the release suite) |

Effect on settlement: when a settling acceptance routes to a vault that is no
longer trusted, the Bridge credits the depositor directly via
`bank.increaseBalances` (the initiation fee is then NOT charged). This is the
fee fallback; the position still registers.

Effect on release (at `d20cf74ba`): the release target must satisfy
`vault == address(0) || (isVaultTrusted[vault] && vault != reservationVault)`
(SM-V1, `tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1430`); a released deposit then becomes an ordinary
pool deposit and can be swept to the ordinary vault by the standard sweep path.

## Storage fields each transition writes

| ID | Record | Fields | Citation |
|---|---|---|---|
| SM-S1 | Position record (`ReservationRequest`) | `owner`, `mintedAmount`, `acceptedAt`, `walletPubKeyHash`, `anchorAmount`, `expiresAt`, `anchorTxHash`, `anchorTxOutputIndex`, `state`, `requestNonce`, `dissolutionEligibleAt`, `cumulativeReanchorFee`, `reanchorCooldownUntil`. `retryCredit` is declared but never written true in M1. | `Reservation.sol:146` |
| SM-S2 | Action record (`ReservationAction`) | `targetWalletPubKeyHash`, `requestedAt`, `timeoutAt`, `txMaxFee`, `actionType`, `state`, `amount`, `minAmount`, `termSeconds`, `dissolutionDelay`, `sourceAnchorUtxoHash`, `thirdPartyHold`. M2-only fields (`feePaid`, `redeemer`, `actionDataHash`, `watchtower*`, `retryCreditSourceNonce`, `isPartial`) are zero in M1. | `Reservation.sol:252` |
| SM-S3 | Per-wallet info (`WalletReservationInfo`) | `amount`, `count` - reserved at request time, released on timeout/settlement/stranding. This ledger counts **reserved capacity**, not proven custody; the wallet lifecycle routes on `walletAnchoredInfo` instead (INV-211). | `tbtc-v2@c8350d109:solidity/contracts/bridge/BridgeState.sol:473` |
| SM-S4 | Global counters, and per-wallet mappings | Global: `activeReservationsCount`, `reservationTotalAmount`, `pendingReservedDeposits`. Per wallet: `pendingThirdPartyReanchorHolds[w]` (INV-205), `walletReservationInfo[w]` (INV-203/204) and `walletAnchoredInfo[w]` - proven anchors only, six mutation sites, `tbtc-v2@c8350d109:solidity/contracts/bridge/BridgeState.sol:519-524`, conservation in INV-211. | `tbtc-v2@c8350d109:solidity/contracts/bridge/BridgeState.sol:437`, `:395`, `:433`, `:517`, `:473`, `:524` |
| SM-S5 | Pending deposit record (`PendingReservedDeposit`) | `isReserved`, `walletPubKeyHash`, `refundDeadline`, `refundDeadlineValidated` - set at reveal (SM-D1), cleared on acceptance (SM-D2) / staleness (SM-D3/SM-D4) / release (SM-D5 at `d20cf74ba`). | `BridgeState.sol:36` |

## keep-core handling of a released deposit

The keep-core evidence is pinned at two commits. At `keep-core@f99ef96e3` the
ordinary deposit-sweep task had no release-to-pool awareness of its own: it
read each deposit's vault from the reveal event (`event.Vault`,
`keep-core:pkg/tbtcpg/deposit_sweep.go:252`), and the reserved-deposit filter
keyed off `depositTargetsReservationVault(event.Vault,
reservationParams.ReservationVault)` (`keep-core:pkg/tbtcpg/reservation_acceptance.go:1228`). After
a release-to-pool, the on-chain `deposits[key].vault` is the re-routed ordinary
vault (or zero) but the `DepositRevealed` event that keep-core replays still
carries the original reveal-time vault. The sweep proposal path resolves vault
from the on-chain `depositRequest.Vault` (`keep-core:pkg/tbtcpg/deposit_sweep.go:333`), so a
released deposit WAS swept to its re-routed vault by keep-core at
`f99ef96e3` by reading, but this had no dedicated keep-core test
(`holds-untested`); the on-chain half is pinned by the release suite at
`tbtc-v2@d20cf74ba`.

The release binding landed on the keep-core release branch in two commits that
`f99ef96e3` predates: `8a714f2c1` regenerated the vendored
`ReservationRouter.fallback-artifact` from the tbtc-v2 release-branch artifact
(adding `releaseReservedDepositToPool(uint256, address)` and
`ReservedDepositReleasedToPool(uint256, address, address)` to the ABI), and
`4b95f0a06` added the post-release sweep regression tests. `4b95f0a06` pins the
current post-release sweep behaviour: the leader's `FindDepositsToSweep`
includes a released deposit (its reveal event still names the reservation vault
but the stored deposit request has been re-routed and the reserved mark
cleared) and proposes it with the stored release-target vault, not the
immutable event vault; the follower's `ValidateDepositSweepProposal` accepts
that proposal because its reservation check gates on the live
`IsReservedDeposit` state, not the event vault. Cite:
`keep-core@4b95f0a06:pkg/tbtcpg/deposit_sweep_release_test.go:91`
(`TestFindDepositsToSweep_IncludesReleasedDeposit`),
`keep-core@4b95f0a06:pkg/tbtc/deposit_sweep_release_test.go:21`
(`TestValidateDepositSweepProposal_AcceptsReleasedDeposit`). Status `holds` at
`4b95f0a06`; the `f99ef96e3` read path was correct but `holds-untested`.

## Findings

- None with status `violated`. The only structural gap is by design:
  `closeReservation` (`Reservation.sol:1141`) is unreachable in M1 (no
  caller), consistent with variant B. This is documented as accepted scope,
  not a defect.

## Doc drift

Drift is recorded against the prior (pre-M1) specs in this directory
(`requirements.md`, `m1-b-implementation.md`, `architecture.md`,
`roadmap.md`). Line numbers refer to those files at their current state.

| Doc path:line | What it says | What the code does |
|---|---|---|
| `m1-b-implementation.md:246` | Cites the acceptance occupancy check at `tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:925-930` | The occupancy check is in `reserveAcceptanceCapacity` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1005`), called from `requestReservationAcceptance` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:621`); `tbtc-v2:solidity/contracts/bridge/Reservation.sol:925` is inside `requestReservationReanchor` |
| `m1-b-implementation.md:248` | Cites `maxActiveReservations > 0` launch gate at `tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1359-1361`/`tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1382-1385` | The gate is `updateReservationCaps` at `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1501` and the enabling require in `updateReservationParameters` at `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1405`; `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1359` is the doc-comment region |
| `m1-b-implementation.md:252` | Cites `validateReservationCapsInvariant` at `tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1217-1231` and its callers at `tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1330-1334`/`tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1392-1396` | The invariant is at `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1322`; its two callers are `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1448` (parameters) and `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1511` (caps) |
| `requirements.md:725` | `maxActiveReservations` "Local/test: 100" | The local test fixture at `9d2086965` sets the local caps to the dust threshold (see commit subject); the "100" launch value is a planning number, not a code value |
| `requirements.md:435` / `requirements.md:908` / `requirements.md:909` | Cite the Decision-1 invariant and the launch gate at `tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1220-1406` / `tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1215-1232` / `tbtc-v2@9f8f5ef1:solidity/contracts/bridge/Reservation.sol:1383-1385` | The invariant is `validateReservationCapsInvariant` at `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1322`; the callers are `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1448`/`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1511`; the caps launch gate is `tbtc-v2:solidity/contracts/bridge/Reservation.sol:1501` |
| `architecture.md` (deploy ordering section) | Describes `maxReservationsPerWallet` "hard-enforced == 1" by the mainnet calldata script | That is the deploy-script check, not an on-chain require; on-chain `updateReservationParameters` only requires `maxReservationsPerWallet > 0` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1398`) |

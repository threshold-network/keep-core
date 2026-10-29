# Verification observations - adversarial adjudication

These observations came from the section drafts, not a new code-fix task.
A separate read-only skeptic attempted to refute each claim.
Integration then inspected the immutable source blobs for the historical ABI claim.
VF-10 to VF-13 record the delta review against the M1 code as then published
(`tbtc-v2@4c98e6b82`, keep-core `f03cc61dc`); the items they correct keep their
earlier text as labelled history. A later keep-core head, `4ced1c839`, adds the
client-side repair that VF-10 records, and it is the head the freeze cites.
No code fix, test run or CI result is implied by this ledger.

## Verdicts

| ID | Draft claim | Verdict | Retained conclusion |
|---|---|---|---|
| VF-01 | Live-cap recheck during re-anchor signing is a confirmed defect | Rejected as a defect at `9d2086965`; resolved by D31 at tbtc-v2 `806b1106f` | At `9d2086965` signing explicitly checked current target capacity, and governance could block a pending proposal by lowering caps. At `806b1106f` the signing-time target-cap recheck is removed (see "Resolution at `806b1106f`" below). |
| VF-02 | Ordinary Bank credit to the reservation vault can bypass reservation provenance | Confirmed, narrowed | A trusted but miswired vault can process pooled sweep credit with no reservation key. This demonstrates wrong provenance/fee handling, not fabricated BTC. |
| VF-03 | Force-stale can destroy a current Pending acceptance before timeout | Rejected | The current Pending-action guard rejects the proposed call. |
| VF-04 | Immediate owner setters bypass a reservations timelock | Rejected as a defect | Owner setters and delayed proxy upgrades are different authority paths. Owner compromise remains a trust-model exposure. |
| VF-05 | Genesis scans establish attacker-amplified client DoS | Rejected as framed | Full-history RPC compatibility is an operational risk. No ordinary attacker-controlled amplification was demonstrated. |
| VF-06 | ABI gate pin and committed fallback disagree | Source mismatch confirmed by integration | The historical router lacks the hold-cap view present in the client fallback. A complete gate execution was not run. |
| VF-07 | Closing finalization has no automated client caller | Confirmed, narrowed | Manual/generated callers exist. Re-anchor from Closing is supported. Absence of automation is not absence of recovery. |
| VF-08 | Gas failures have no detection before late windows expire | Rejected | Submission failure counters and overdue notifications can signal earlier. They do not forecast every gas-price problem. |
| VF-09 | Release destination exclusion remains true forever | Confirmed, narrowed | The exclusion is checked at release time. Governance can later select that destination as the configured vault. The released flag remains false. |
| VF-10 | The recorded stranding policy matches the wallet-state guards and the settlement hook | Confirmed drift, corrected | At `tbtc-v2@4c98e6b82` stranding admits `Terminated`/`Closed` always, a `Live` wallet past `dissolutionEligibleAt`, and `Closing` past both `dissolutionEligibleAt` and the closing period; settlement strands only `Closed`/`Terminated`. The earlier text denied the Live path, omitted the closing period and listed Closing as a settlement-strand target. The published code and the intended policy agree: a position on a healthy (Live) wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was taken on 2026-10-29 and the code does not change. |
| VF-11 | The wallet lifecycle routes on the reservation count | Confirmed drift, corrected | `moveFunds` and the closing threshold read the proven-anchor ledger `walletAnchoredInfo`; only the final `Closing->Closed` gate reads `walletReservationInfo`. Pending-only capacity therefore routes to Closing. |
| VF-12 | The acceptance request takes a reservation key and a wallet | Confirmed drift, corrected | The published request takes the funding transaction, its SPV proof, the output index and the wallet, and derives the key from the proven hash. The generated keep-core binding agrees; no production keep-core caller exists. |
| VF-13 | A withheld signature cannot cost the depositor anything because a late anchor settles | Confirmed, narrowed | Late acceptance settles only through `timeoutAt + termSeconds`; late re-anchor stays unbounded. Past the acceptance cutoff the anchor is not credited by that path, which is the limit AR-10 already records. |

## Evidence and limits

### VF-01: current caps at signing, snapshots at settlement

Request reserves target count and amount (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:878-903`).
The signing validator reads current caps and compares already-reserved occupancy using `<=`
(`tbtc-v2:solidity/contracts/bridge/WalletProposalValidator.sol:1155-1183`).
Its documented signing requirements include current target capacity (:1087-1089).
Settlement of confirmed Bitcoin spends does not apply the same request throttles.
These are distinct policy boundaries, not evidence of double reservation.

Design review question: should governance preserve signing of every pending authorization when it lowers a cap?
The inspected code does not promise that property.

Resolution at `806b1106f` (D31): the signing-time target-cap recheck is removed.
The validator doc block says target count and amount capacity are enforced once,
at request time, and are not repeated at signing
(`tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1092-1098`).
`validateReservationReanchorProposal` is at
`tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1115`.
The target Live check stays
(`tbtc-v2@806b1106f:solidity/contracts/bridge/WalletProposalValidator.sol:1156-1160`).
A lowered target cap, or capacity consumed by another request after this one
was accepted, therefore no longer blocks signing of a pending re-anchor.
Request-time caps still apply to third-party requests and to governance from a
Live source.
Evidence:
- Mocked-bridge validator test, above-cap target passes:
  `tbtc-v2@806b1106f:solidity/test/bridge/WalletProposalValidator.test.ts:4322-4382`.
- Real-validator harness (compiled validator equal to `806b1106f`) accepts a
  target at the count cap (`keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1193`),
  above the count cap (`keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1205`)
  and above the amount cap (`keep-core@d40f6aadc:pkg/chain/ethereum/tbtc_validator_harness_test.go:1219`).
  At the original client pin the harness rejected the above-cap target
  (`keep-core:pkg/chain/ethereum/tbtc_validator_harness_test.go:1191`).

The paragraphs above describe the original adjudication at `9d2086965`.
The design question is answered for target caps: pending authorizations are no
longer blocked at signing by a lowered cap. Nothing is deployed on Sepolia or mainnet.

### VF-02: pooled backing is not an invented reservation anchor

`receiveBalanceIncrease` is Bank-only and has no reservation-key argument
(`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:147-195`).
An ordinary proven sweep can route credit to a trusted supplied vault
(`tbtc-v2:solidity/contracts/bridge/DepositSweep.sol:230-245`).
The ordinary sweep rejects deposits whose underlying reserved flag is true (:432-444).
If vault V is trusted while the configured reservation vault is not V,
a normal deposit to V can reach the reservation credit handler.
That handler uses existing Bank credit; `TBTCVault.mint` consumes Bank balance
(`tbtc-v2:solidity/contracts/vault/TBTCVault.sol:85-95`).
The demonstrated risk is reservation provenance and fee routing, not arbitrary unbacked minting.
Do not treat the code's known-gap comment as a completed mitigation or an approval decision.
Human disposition (D32, 2026-10-12): accepted for the Sepolia drill only; an on-chain guard must land before mainnet (AR-16).

### VF-03: force-stale preserves current authorization

`forceStaleReservedDeposit` checks unswept funding and rejects a current Pending action
(`tbtc-v2:solidity/contracts/bridge/Reservation.sol:1289-1307`).
Its deadline override is an intentional governance anti-grief mechanism.
The proposed pre-timeout destruction of a Pending acceptance reverts with
"Acceptance authorization pending".

### VF-04: authority paths are not interchangeable

`setVaultStatus` and `setSpvMaintainerStatus` are immediate owner operations
(`tbtc-v2:solidity/contracts/bridge/BridgeGovernance.sol:308-321`).
Begin/finalize delays apply to the parameter ceremonies that implement them.
A proxy-admin timelock is not a blanket delay on every BridgeGovernance owner call.
Actual owner composition and key-loss tolerance are deployment facts.

### VF-05: provider history limits

The target search has a recent-range search and a full-history fallback.
The below-dust history check also asks for old wallet-specific events
(`keep-core:pkg/tbtcpg/reservation_reanchor.go:1237-1295`, `:1529-1591`).
Full-history limits can abort a pass. Repeated passes can repeat work.
The fallback is not guaranteed to run only once for the lifetime of a wallet.
A malicious RPC can refuse other essential reads too; this alone does not establish amplification.

### VF-06: immutable source mismatch, not an observed CI failure

The workflow pins `TBTC_V2_REF` to `eec999aad43b3913df378840773348e17f68f1ba`
(`keep-core:.github/workflows/client.yml:206`).
Integration read that router source and the client fallback at `f99ef96e3`.
The former lacks `pendingThirdPartyReanchorHolds`; the latter includes it.
The whole-ABI comparison therefore has an incompatible input pair.
A full compile and `verify-vendored-fallback` execution was not performed in this documentation task.
The public PR head (`2b63ae60d`) is consistent: its fallback has no `pendingThirdPartyReanchorHolds`.
The mismatch exists on the keep-core reservations branch, because the hold-cap commit `6aac91585` is not an ancestor of the spec pin.
Human disposition (D33, 2026-10-12): bump `TBTC_V2_REF` together with the vendored harness validator,
in the keep-core change that brings the hold-cap and D31 bindings to the epic pull request, once the tbtc-v2 branch carries that commit.
Run the gate locally against that commit before the push.
That change is now on the reservations branch: `TBTC_V2_REF` names the tbtc-v2 reservations branch head,
and the vendored fallbacks, the Go bindings and the harness validator were checked against the artifacts
compiled at that commit ([approved design deltas](13-approved-design-deltas.md)).

The draft also falsely attributed D26 bindings to `f99ef96e3`.
That historical binding contains no `releaseReservedDepositToPool`.
`8a714f2c1` adds the binding; `4b95f0a06` adds release/sweep regressions.
XR-F-22 now identifies those later revisions explicitly.

### VF-07: Closing is recoverable through explicit calls

The client re-anchor task accepts both MovingFunds and Closing
(`keep-core:pkg/tbtcpg/reservation_reanchor.go:170-174`).
The normal main-funds proof enters Closing directly
(`tbtc-v2:solidity/contracts/bridge/Wallets.sol:438-441`, `:649-660`).
Final closing requires elapsed time and zero reservation count (:363-389).
Manual/generated finalize callers exist, but no automatic production task was identified.
Do not describe normal completion as permanently stuck MovingFunds.

### VF-08: failures can be visible before proof expiry

The shared proof submitter records a failed submission
(`keep-core:pkg/maintainer/spv/reservation_reanchor_proof.go:285-299`).
The timeout watcher also records overdue actions before a successful notification
(`keep-core:pkg/maintainer/spv/reservation_action_timeout_watch.go:459-555`).
A timeout event requires a successful transaction; it does not appear automatically.
Gas-specific predictive telemetry was not demonstrated, but total absence of detection is false.

### VF-09: point-in-time destination validation

D26 rejects the configured reservation vault as its destination at execution
(`tbtc-v2@d20cf74ba:solidity/contracts/bridge/Reservation.sol:1428-1433`).
It clears reserved state and writes the destination vault (:1441-1446).
A later governance update can select that vault when both outstanding counters are zero (:1539-1555).
The released deposit stays non-reserved; this does not recreate a reservation or duplicate its counters.
A resulting ordinary credit can expose the routing issue described in VF-02.

### VF-10: stranding and settlement wallet states

The published code and the intended policy agree: a position on a healthy (Live)
wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was
taken on 2026-10-29 and the code does not change.

The guards are explicit at the published target.
`notifyReservationStranded` requires an `Active` position and one of:
`Terminated`, `Closed`, `Live` with `block.timestamp >= dissolutionEligibleAt`,
or `Closing` with `dissolutionEligibleAt` and
`block.timestamp >= closingStartedAt + walletClosingPeriod`
(`tbtc-v2@4c98e6b82:solidity/contracts/bridge/Reservation.sol:1301-1326`).
`MovingFunds` is not an admissible state.
The settlement hook strands only on `Closed` and `Terminated`
(`tbtc-v2@4c98e6b82:solidity/contracts/bridge/ReservationProofs.sol:243-253`),
so a `Closing` target keeps the position `Active` and the owner keeps the
re-anchor right that `requestReservationReanchor` grants from a Closing source.
Both properties are pinned by tests rather than inferred:
`Bridge.ReservationOccupancy.test.ts:186` (Closing rejected),
`:222` (closing period not elapsed), `:263` and `:313` (inclusive boundary),
`:386` and `:397` (Live before and after eligibility), and
`ReservationProofs.test.ts:1434` (Closing not stranded).

Items rewritten: SM-R6, SM-R7, SM-R8, SM-W5, EC-11, DR-02, AR-11, INV-103,
INV-104, INV-112, INV-603, TM-07, US-14; US-101 was added for the Live branch.
The client side was recorded, not repaired, at the head then published: that
watcher dispatched `Closed`/`Terminated`/`Closing` and never `Live`, and its
Closing check read dissolution eligibility only
(`keep-core@f03cc61dc:pkg/maintainer/spv/reservation_stranding_watch.go:148-162`,
`:282-291`), so an early notification reverted and waited for a later pass.
keep-core #4365, in `4ced1c839`, repairs both halves: the watcher dispatches all
four admissible states
(`keep-core@4ced1c839:pkg/maintainer/spv/reservation_stranding_watch.go:155-176`)
and holds a `Closing` wallet until `closingStartedAt + walletClosingPeriod`
(`keep-core@4ced1c839:pkg/maintainer/spv/reservation_stranding_watch.go:379-392`).

### VF-11: the proven-anchor ledger

`BridgeState` carries two per-wallet ledgers
(`tbtc-v2@4c98e6b82:solidity/contracts/bridge/BridgeState.sol:473`, `:519-524`).
`walletReservationInfo` counts reserved capacity from request time;
`walletAnchoredInfo` counts only `Active` or `ActionPending` positions whose
anchor is proven to sit at the wallet.
`moveFunds` routes on the second (`Wallets.sol:629-631`) and
`notifyWalletCloseable` adds its amount to the closing threshold (`:350-356`),
while `notifyWalletClosingPeriodElapsed` still requires a zero
`walletReservationInfo` count (`:392-395`).
Consequences: a wallet with only pending capacity closes immediately and never
enters the slash-prone moving-funds window, and a wallet holding a proven
anchor cannot close underneath it.
Tests: `Reservation.test.ts:2007` and `:2057`.

Items rewritten: SM-W1, SM-W2, SM-W3, SM-S3, SM-S4, US-34, INV-102, INV-104,
TM-08; INV-211 was added for the ledger's conservation.
The storage cost is one slot taken from the `__gap`, already reflected in
`BridgeState`'s budget comment.

### VF-12: the funding proof belongs to the acceptance request

`requestReservationAcceptance(fundingTx, fundingProof, fundingOutputIndex, walletPubKeyHash)`
validates the proof and derives the key from
`keccak256(abi.encodePacked(fundingTxHash, fundingOutputIndex))`
(`tbtc-v2@4c98e6b82:solidity/contracts/bridge/ReservationRouter.sol:230-247`).
`revealDeposit` checks no proof, so this is the step that makes fabricated
reserved deposits cost a real Bitcoin confirmation.
The SDK assembles the proof at `txProofDifficultyFactor()`
(`tbtc-v2@4c98e6b82:typescript/src/services/reservations/reservations-service.ts:539-563`,
test `typescript/test/services/reservations.test.ts:1017`).
The generated keep-core binding carries the same four arguments and the
selector `0x19821f16`
(`keep-core@4ced1c839:pkg/chain/ethereum/tbtc/gen/abi/ReservationRouter.go:895-911`);
no production keep-core caller sends it, and the depositor is the only
authorized caller (`msg.sender == deposit.depositor`).
The proof is required at acceptance, not at reveal, so it does not by itself
close reveal-time griefing.

Items rewritten: US-03, US-97, XR-F-17, SM-A1, SM-C2, and the coverage-table
row for the entry point.

### VF-13: the late-acceptance cutoff is the no-loss limit

A timed-out acceptance settles late only while
`block.timestamp <= uint256(action.timeoutAt) + action.termSeconds`
(`tbtc-v2@4c98e6b82:solidity/contracts/bridge/ReservationProofs.sol:204-211`,
revert "Late acceptance settlement window expired",
test `ReservationProofs.test.ts:446`).
Late re-anchor settlement has no equivalent bound.
TM-16's unconditional "a late confirmed anchor still settles late, so no funds
are lost" therefore overstated the recovery path for acceptances; the bounded
statement now matches AR-10, and TM-04's residual note distinguishes the two
action types instead of presenting one window for both.

## Rejected stronger claims

- Ordinary pooled credit implies arbitrary unbacked TBTC creation.
- A force-stale call can clear a current Pending acceptance.
- Every administrative call inherits a proxy-upgrade timelock.
- Source inspection is an observed CI failure.
- Historical client bindings already contain the later release entry point.
- A MovingFunds proof leaves the wallet permanently MovingFunds with no stranding route.
- Passing a term, timeout or cooldown causes an automatic transaction.
- A confirmed acceptance spend can always use the original funding refund script.

Every remaining design question belongs to the review queue, not an implicit scope change.

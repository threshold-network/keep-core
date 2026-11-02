# G1 (go-signing) Triage Report

Evidence note: the solidity-auditor scan output and x-ray report named in this file are an internal run
record and are not part of this record. The `evidence/g1/` scratch tests referenced here are in this tree
(`evidence/`, committed with the report).

Scan root: (run record not published; stamp `20261001-101411`)

## Scope

Go client code in `keep-core` that builds and validates wallet signing proposals for the tBTC v2
UTXO-reservation epic. The PR diff (`git diff a7ac8989 49c4df13d`) is the review target. Files:

- `pkg/tbtcpg/reservation_acceptance.go`
- `pkg/tbtcpg/reservation_reanchor.go`
- `pkg/tbtcpg/fee.go`
- `pkg/tbtcpg/deposit_sweep.go`
- `pkg/tbtc/reservation.go`
- `pkg/tbtc/proposal_fee_check.go`
- `pkg/tbtc/deposit_reveal_lookup.go`
- `pkg/tbtc/marshaling.go`
- `pkg/tbtc/deposit_sweep.go`
- `pkg/tbtc/coordination.go`
- `pkg/tbtc/node_proposals.go`

On-chain guards consulted (pinned contracts): `Bridge.sol`, `ReservationRouter.sol` / `Reservation.sol`,
`ReservationVault.sol`, `WalletProposalValidator.sol` (in `tbtc-v2@eec999aad:solidity/contracts/`). Threat
model: a malicious coordination leader is in the threat model; a follower must not co-sign a proposal that
is unsafe once co-signed.

## Counts

- Findings: **13 total** (11 main F rows + 3 supplement S rows, both G1 rows confined to reservation code).
  Confirmed 4 (F6, F7, F8, S3), Plausible 5 (F2, F11, F12, S1, S2), Disputed 3 (F1, F4, F5), Duplicate 1
  (F9 of F5).
- Severity (Confirmed/Plausible only): Medium 6 (F2, F6, F7, F8, F11, F12), Low 3 (S1, S2, S3).
- Leads: the per-source lead table is not published; the leads the fix plan references are listed in the
  index's appendix.

## Summary table

| ID | Severity | Verdict | Origin | Title | Location |
|----|----------|---------|--------|-------|----------|
| G1-F1 | Medium | Disputed | Introduced by PR #4282 | Vault rotation makes a reserved deposit un-sweptable | `tbtcpg.findDeposits` (`pkg/tbtcpg/deposit_sweep.go:252`) |
| G1-F2 | Medium | Plausible | Introduced by PR #4282 | Re-anchor resume signs to an attacker-chosen target | `tbtcpg.ReservationReanchorTask.Run` (`pkg/tbtcpg/reservation_reanchor.go:308`) - Second check: Plausible, held: resume passes `action.TargetWalletPublicKeyHash` with no trust gate; permissionless on-chain request; requires attacker wallet + source MovingFunds/Closing + coordination leader. |
| G1-F4 | Low | Disputed | Introduced by PR #4282 | Follower anchor execution never re-checks funding confirmations | `tbtc.reservationAnchorAction.execute` (`pkg/tbtc/reservation.go:583`) |
| G1-F5 | Low | Disputed | Introduced by PR #4282 | Re-anchor assembler omits the on-chain dust floor | `tbtc.AssembleReservationReanchorTransaction` (`pkg/tbtc/reservation.go:409`) |
| G1-F6 | Medium | Confirmed | Introduced by PR #4282 (gap pre-existing) | Below-relay sweep fee is co-signed by every follower | `tbtc.depositSweepAction.execute` (`pkg/tbtc/deposit_sweep.go:258`) - Second check: Confirmed, held: `ValidateDepositSweepProposal` generated binding has no ctx/deadline; on-chain validator only enforces `> 0`. |
| G1-F7 | Medium | Confirmed | Introduced by PR #4282 | Below-relay anchor fee is co-signed by every follower | `tbtc.reservationAnchorAction.execute` (`pkg/tbtc/reservation.go:583`) - Second check: Confirmed, held: `ValidateReservationAnchorProposal` has no ctx deadline; on-chain only enforces `> 0`; same root as G1-F6. |
| G1-F8 | Medium | Confirmed | Introduced by PR #4282 | Below-relay re-anchor fee is co-signed by every follower | `tbtc.reservationReanchorAction.execute` (`pkg/tbtc/reservation.go:719`) - Second check: Confirmed, held: `validateReservationReanchorProposal` has no relay floor; same root as G1-F6. |
| G1-F9 | Low | Duplicate of G1-F5 | Introduced by PR #4282 | Re-anchor assembler omits the on-chain dust floor | `tbtc.AssembleReservationReanchorTransaction` (`pkg/tbtc/reservation.go:409`) |
| G1-F11 | Medium | Plausible | Introduced by PR #4282 | Unbounded validation RPC call locks a wallet's dispatcher forever | `tbtc.walletAction.execute` / reservation paths (`pkg/tbtc/reservation.go:492`) - Second check: Plausible, held: dispatcher lock confirmed (map stores action before goroutine launch, removes only after return); generated binding calls have no ctx deadline. |
| G1-F12 | Medium | Plausible | Introduced by PR #4282 | Re-anchor target search picks an untrusted wallet | `tbtcpg.ReservationReanchorTask.reanchorTargetSearch.pick` (`pkg/tbtcpg/reservation_reanchor.go:927`) - Second check: Plausible, held: root cause shared with G1-F2; first Live wallet selected, no trust gate. |
| G1-S1 | Low | Plausible | Introduced by PR #4282 | Re-anchor task gates signing windows on node wall clock, not on-chain timestamps | `tbtcpg.ReservationReanchorTask.Run` (`pkg/tbtcpg/reservation_reanchor.go:288,:344`) |
| G1-S2 | Low | Plausible | Introduced by PR #4282 | Acceptance task gates its timeout checks on node wall clock, not on-chain timestamps | `tbtcpg.ReservationAcceptanceTask.findReservationAcceptanceCandidates` (`pkg/tbtcpg/reservation_acceptance.go:453,:529`) |
| G1-S3 | Low | Confirmed | Introduced by PR #4282 | Follower fee-floor warning fires on every honest low-fee proposal | `tbtc.warnIfProposedWalletTxFeeBelowBufferedFloor` (`pkg/tbtc/proposal_fee_check.go`) vs `tbtcpg.applyWalletTxFeeFloor` (`pkg/tbtcpg/fee.go`) |

## Findings

### G1-F1: Vault rotation makes a reserved deposit un-sweptable

Verdict: **Disputed** · Severity: **Medium** (liveness) · Origin: **Introduced by PR #4282**

`findDeposits` (`pkg/tbtcpg/deposit_sweep.go:252`) only calls `IsReservedDeposit` for a revealed deposit
when the deposit's reveal-time vault equals the *current* reservation vault. After a governance vault
rotation, a deposit that was revealed reserved under the old vault no longer matches, so it is treated as
an ordinary sweepable deposit and put in the sweep candidate list. Because its on-chain `isReserved` flag
persists across the rotation, the on-chain validator rejects the sweep that includes it, and the wallet's
deposit sweep stalls until that deposit is accepted or its refund deadline elapses.

The on-chain guard that stops any fund move is `WalletProposalValidator.validateDepositSweepProposal`,
which requires `!bridge.isReservedDeposit(depositKey)` for every deposit in the proposal. That guard makes
this a liveness stall, not a custodial risk: the reserved deposit is never swept. The finding's proposed
fix (drop the vault-match pre-filter and check every revealed deposit via `IsReservedDeposit` so reserved
ones are *skipped* from the candidate list) is a valid liveness improvement, but it is not a security fix.
There is no fund-move path to close on the Go side.

- Real consequence: a wallet that still has unreleased old-vault reserved deposits cannot complete its
  deposit sweep after a vault rotation (stall, self-recovering when the deposit is accepted/stales).
- Fix (liveness): in `findDeposits`, when `reservationsActive`, run `IsReservedDeposit` on every revealed
  deposit (not just vault-matching ones) and skip the reserved ones; the on-chain `!isReservedDeposit`
  check remains the backstop.

### G1-F2: Re-anchor resume signs to an attacker-chosen target

Verdict: **Plausible** · Severity: **Medium** · Origin: **Introduced by PR #4282**

`ReservationReanchorTask.Run` (`pkg/tbtcpg/reservation_reanchor.go:308`) reads
`action.TargetWalletPublicKeyHash` from the on-chain action record and calls
`ProposeReservationReanchor(..., action.TargetWalletPublicKeyHash, 0)`. Any unprivileged EOA can call
`requestReservationReanchor` (ReservationRouter external entry point, no signer gate) to set the re-anchor
target to their own Live wallet, when the source wallet is in MovingFunds or Closing state. The
permissionless re-anchor is by design on-chain, but the Go client's resume path blindly co-signs whatever
target was authorized on-chain.

A malicious coordination leader is in the threat model; followers must not co-sign a proposal that is
unsafe once co-signed. If the attacker registers their own Live wallet and calls
`requestReservationReanchor` to direct a retiring wallet's reserved anchors to it, the operator's resume
task co-signs moving those anchors to the attacker's custody. The attacker's wallet then custodies those
depositors' collateral, and the depositor's recovery is delayed pending the attacker's eventual
moving-funds sweep or wallet termination.

- Attack path: (1) attacker registers their own Live wallet; (2) target wallet enters MovingFunds/Closing;
  (3) attacker calls `requestReservationReanchor` to set target to their own wallet; (4) operator's
  reanchor task resumes and co-signs the re-anchor to the attacker's wallet. The on-chain validator only
  requires `Live` state for the target; it does not gate by custodian trust.
- Real fix: before resuming, apply the same custody gate as the target search to the on-chain target.
  Specifically, in the resume path, call `isTargetWalletTrusted(action.TargetWalletPublicKeyHash)` using
  the same criteria as `reanchorTargetSearch.pick` (e.g., wallet was registered at least
  `minWalletAgeBlocks` ago, or is on an operator-configured allowlist). If the target is untrusted, skip
  the resume rather than sign. This is the same gate that fixes G1-F12; implementing it in one place and
  sharing it covers both paths. Decision 2026-10-05: use the on-chain age gate (CreatedAt +
  walletCreationPeriod <= chain head time); no allowlist.
- Fix update (2026-10-05): the check that matters is the follower one. Add the on-chain age check to
  `reservationReanchorAction.execute` (`pkg/tbtc/reservation.go:692-777`). A malicious leader can skip any
  leader-side check, so this is where it has to live. The follower rejects a target when CreatedAt +
  walletCreationPeriod is later than the chain head time plus a 2-block tolerance. The leader-side resume
  and `pick` checks are strict and only stop honest leaders from wasting proposals. See
  `recommendations-pr-4282.md` P3.
- Example real fix diff:
```diff
 // In ReservationReanchorTask.Run, before calling ProposeReservationReanchor on resume:
+ targetKey := action.TargetWalletPublicKeyHash
+ registeredAt, err := rat.chain.GetWalletRegisteredAt(targetKey)
+ if err == nil && rat.isTargetWalletTrusted(targetKey, registeredAt) {
+   // trusted: proceed with resume
+ } else {
+   taskLogger.Warnf("re-anchor target [0x%x] does not satisfy custody gate, "+
+       "skipping resume", targetKey)
+   continue
+ }
```
Second check: Plausible, held. Resume path (`reservation_reanchor.go:308`) passes
`action.TargetWalletPublicKeyHash` to `ProposeReservationReanchor` with no trust gate. On-chain
`requestReservationReanchor` (Reservation.sol:723) is permissionless for MovingFunds/Closing source
wallets. Attacker must control source wallet signing group to authorize the initial request, but once
authorized any EOA can repeatedly re-request to the same target. Severity Medium held: requires attacker
Live wallet + source in MovingFunds/Closing + coordination leader role.

### G1-F4: Follower anchor execution never re-checks funding confirmations

Verdict: **Disputed** · Severity: **Low** · Origin: **Introduced by PR #4282**

`reservationAnchorAction.execute` (`pkg/tbtc/reservation.go:583`) fetches the funding UTXO and the
on-chain deposit request but does not call `GetTransactionConfirmations`. A leader who proposes a
reservation acceptance for a funding transaction that has fewer than 6 on-chain Bitcoin confirmations
would have that proposal pass the leader's confirmation check (G-38, `reservation_acceptance.go:890`), get
signed by every follower, and be broadcast.

The on-chain SPV proof settlement (`ReservationProofs.submitReservationAcceptanceProof` ->
`validateProof`) validates the anchor transaction against actual Bitcoin history. If the funding
transaction never confirmed, the anchor transaction also never confirms, and the proof reverts at
settlement time. No tBTC is minted and no custody is transferred. The follower's missing confirmation
re-check is a defense-in-depth gap; the on-chain SPV proof is the final backstop. The confirmation count
only increases over time, so a leader's confirmed-check at proposal time carries forward unless a deep
reorg occurs (6+ block reorg, astronomically rare).

- Attack path: a malicious leader with a lying Electrum backend could get followers to sign a reservation
  acceptance anchored to an unconfirmed funding UTXO. The signed transaction is broadcast but never
  confirms; the on-chain proof settlement reverts; no value moves.
- Fix (hardening): add `GetTransactionConfirmations` to `reservationAnchorAction.execute` to independently
  verify the 6-confirmation gate before signing.

### G1-F5: Re-anchor assembler omits the on-chain dust floor

Verdict: **Disputed** · Severity: **Low** · Origin: **Introduced by PR #4282** (hardening)

`AssembleReservationReanchorTransaction` (`pkg/tbtc/reservation.go:409`) checks
`if anchorValue <= 0 { return error }`. The on-chain settlement (`ReservationProofs.validateReservationReanchorProof`)
additionally requires `newAnchorAmount > action.txMaxFee` (dust floor). The assembler's `> 0` guard does not
explicitly mirror the `> txMaxFee` dust floor.

However, for any valid generation, the request-time gate (`requestReservationReanchor` requires
`anchorAmount > txMaxFee + minAmount`) combined with the governance invariant
(`reservationMinAmount > reservationTxMaxFee` in `updateReservationParameters`) guarantees
`anchorAmount > txMaxFee + minAmount > 2 * txMaxFee`. Since `fee <= txMaxFee` at assembly time,
`newAnchor = anchorAmount - fee > txMaxFee + minAmount - txMaxFee > minAmount > txMaxFee`. Therefore
`anchorValue > txMaxFee` is always satisfied for a generation that reached the assembler via the normal
request path, and the assembler's `> 0` guard is sufficient.

The finding as described (a signed re-anchor permanently unprovable because anchorValue falls in
`(0, txMaxFee]`) is unreachable in the normal production flow. The assembler's missing explicit dust-floor
guard is a defense-in-depth gap that would matter if the assembler were ever called with an action record
that bypassed the request-time gate.

- Fix (hardening): add an explicit dust-floor check to the assembler:
  `if anchorValue <= int64(action.TxMaxFee) || anchorValue < int64(action.MinAmount) { return errBelowDustFloor }`.
  This mirrors the on-chain `newAnchorAmount > txMaxFee` check and the `minAmount` snapshot guard.

### G1-F6: Below-relay sweep fee is co-signed by every follower

Verdict: **Confirmed** · Severity: **Medium** (liveness) · Origin: **Introduced by PR #4282** (gap
pre-existing)

`depositSweepAction.execute` (`pkg/tbtc/deposit_sweep.go:258`) calls `checkSweepFeeFloor` and only *warns*
(logs + emits a metric) when the proposed fee is below the buffered relay floor. It still co-signs the
proposal. A malicious leader who proposes a fee at or near 1 sat/vB causes the sweep transaction to never
relay; the transaction stalls until the proposal expires. The leader can re-propose a below-relay fee in
every window, permanently jamming the wallet's deposit sweep.

The on-chain `WalletProposalValidator.validateSweepTxFee` enforces only the upper bound
(`depositTxFee + remainder <= depositTxMaxFee`) and that fee > 0; it does not enforce a minimum relay
floor. The off-chain log-only posture is documented as an intentional tradeoff during mixed-version
rollout (the `deposit_sweep.go` comment cites `threshold-network/keep-core#4171`), but it leaves a
liveness vector open. The proposal expiry provides eventual self-healing, but a persistent malicious
leader can re-propose every window indefinitely.

- Attack path: malicious leader proposes a below-relay sweep fee; every follower warns and signs; tx never
  relays; wallet deposit sweep jammed until proposal expiry + new leader takes over.
- Fix: add an on-chain lower bound in `WalletProposalValidator.validateSweepTxFee` (requires a protocol
  upgrade to change the validator), or coordinate a hard floor reject across all nodes before a
  mixed-version window (the code comment at `deposit_sweep.go:673-706` documents this tradeoff). Decision
  2026-10-05: hard reject below the fixed protocol minimum 5 sat/vB x (follower vsize - 1) on all five
  paths; no activation switch. The on-chain relay floor stays a separate structural item, tracked
  privately.

Second check: Confirmed, held. `reservation.go:497` calls `ValidateReservationAnchorProposal` (generated
binding, no ctx deadline). On-chain `requestReservationAcceptance` enforces only
`anchorTxFee <= txMaxFee && > 0` (no relay floor). Below-relay anchor fee is liveness DoS; on-chain
validator does not stop it. Severity Medium held.

### G1-F7: Below-relay anchor fee is co-signed by every follower

Verdict: **Confirmed** · Severity: **Medium** (liveness) · Origin: **Introduced by PR #4282**

`reservationAnchorAction.execute` (`pkg/tbtc/reservation.go:583`) calls
`warnIfProposedWalletTxFeeBelowBufferedFloor` (reservation.go:588) and only *warns* when the proposed
anchor fee is below the buffered relay floor. It still signs. A malicious leader who proposes a
below-relay anchor fee causes the acceptance transaction to never relay; the reservation acceptance stalls
until the proposal expires; the depositor's funding remains unsettled on Bitcoin and the tBTC mint is
delayed.

The on-chain `ReservationRouter.requestReservationAcceptance` settlement requires
`anchorTxFee <= action.txMaxFee` and `anchorTxFee > 0`; no relay-floor minimum. Same root cause as G1-F6.
The below-relay anchor fee is a liveness DoS, not a fund-safety issue; the on-chain validator does not
stop it.

- Attack path: malicious leader proposes a below-relay anchor fee; every follower warns and signs;
  acceptance tx never relays; pending reservation acceptance stalls until expiry.
- Fix: same as G1-F6; an on-chain minimum relay-floor validation is the structural fix; off-chain
  hard-reject requires a coordinated mixed-version upgrade. Decision 2026-10-05: hard reject below the
  fixed protocol minimum 5 sat/vB x (follower vsize - 1) on all five paths; no activation switch.

Second check: Confirmed, held. Same root cause as G1-F6. `validateReservationReanchorProposal` (re-anchor
path) has no relay-floor enforcement. On-chain `requestReservationReanchor` (Reservation.sol:793) enforces
only `reanchorTxFee > 0`. Attacker can jam re-anchor path indefinitely with persistent malicious leader.
Severity Medium held.

### G1-F8: Below-relay re-anchor fee is co-signed by every follower

Verdict: **Confirmed** · Severity: **Medium** (liveness) · Origin: **Introduced by PR #4282**

`reservationReanchorAction.execute` (`pkg/tbtc/reservation.go:719`) calls
`warnIfProposedWalletTxFeeBelowBufferedFloor` (reservation.go:724) and only *warns* when the proposed
re-anchor fee is below the buffered relay floor. It still signs. A malicious leader who proposes a
below-relay re-anchor fee causes the re-anchor transaction to never relay; the reservation remains stuck
in `ActionPending` and the anchor UTXO is not re-anchored; if the source wallet is in MovingFunds, the
depositor's recovery path is blocked until the proposal expires and a new leader re-proposes.

The on-chain `ReservationRouter.requestReservationReanchor` settlement only enforces the upper bound
(`reanchorTxFee <= action.txMaxFee`) and `> 0`; no relay-floor minimum. Same root cause as G1-F6 and
G1-F7. The jamming is bounded by the proposal expiry and eventual self-healing via a new leader, but a
persistent malicious leader can jam the re-anchor path indefinitely for a target reservation.

- Attack path: malicious leader proposes a below-relay re-anchor fee; every follower warns and signs;
  re-anchor tx never relays; reservation stays ActionPending; source wallet MovingFunds window advances
  without the anchor being re-anchored.
- Fix: same as G1-F6; an on-chain minimum relay-floor validation is the structural fix; off-chain
  hard-reject requires a coordinated mixed-version upgrade. Decision 2026-10-05: hard reject below the
  fixed protocol minimum 5 sat/vB x (follower vsize - 1) on all five paths; no activation switch.

Second check: Confirmed, held. Same root cause as G1-F6. `validateReservationReanchorProposal` has no
relay-floor enforcement; on-chain `requestReservationReanchor` enforces only `reanchorTxFee > 0`. Attacker
can jam re-anchor path indefinitely with persistent malicious leader. Severity Medium held.

### G1-F9: Re-anchor assembler omits the on-chain dust floor

Verdict: **Duplicate of G1-F5** · Severity: **Low** · Origin: **Introduced by PR #4282**

This finding has the same title, the same function (`tbtc.AssembleReservationReanchorTransaction`), the
same mechanism (assembler checks `> 0` but not `> txMaxFee` dust floor), and the same fix as G1-F5.
Confidence 78 (vs 80 for G1-F5). Ruled a duplicate of G1-F5.

### G1-F11: Unbounded validation RPC call locks a wallet's dispatcher forever

Verdict: **Plausible** · Severity: **Medium** (liveness) · Origin: **Introduced by PR #4282**

The reservation action execute paths (`pkg/tbtc/reservation.go:492-521`) call
`validateReservationAnchorProposal` / `validateReservationReanchorProposal` (Ethereum view calls) and
`GetTransaction` (Bitcoin RPC) without any context deadline. The `ethclient` HTTP transport has no default
per-call read deadline. The `walletDispatcher` holds a per-wallet busy lock (`wd.actions[key]`) until the
execute goroutine returns. If the Ethereum RPC endpoint is unresponsive (hung connection, no response), the
dispatcher goroutine is blocked indefinitely and the lock is held for the entire duration, effectively
freezing all other signing actions for that wallet until the process is restarted.

`reservation.go:497` (`validateReservationAnchorProposal` call) and `reservation.go:499` (`GetTransaction`
call) are inside the dispatcher goroutine with no timeout wrapper. Other paths in the codebase use
`context.WithTimeout` (e.g., `HeaderByNumber` in current-block reads uses a 30-second timeout), but the
reservation execute paths lack this protection.

The attack path requires an operator's Ethereum RPC endpoint to hang or become unresponsive. A malicious
or negligent RPC provider (bounded-untrusted in the threat model) can cause this. The impact is per-wallet
liveness (all signing actions for that wallet block), not a fund move.

- Attack path: operator's Ethereum RPC endpoint hangs on the validation view call; the dispatcher
  goroutine blocks; the per-wallet busy lock is held; all other signing actions for that wallet (deposit
  sweep, redemption, moving funds) queue or timeout.
- Real fix: wrap the on-chain validator calls and Bitcoin RPC reads in the reservation execute paths with
  a bounded `context.WithTimeout` (e.g., 30 seconds, consistent with other paths). Add a timeout handler
  that releases the dispatcher lock and returns an error on timeout. This mirrors the pattern already used
  for `HeaderByNumber` in the current-block path.

Second check: Plausible, held. `reservation.go:497` calls `ValidateReservationAnchorProposal` (generated
binding, no ctx deadline). `reservation.go:499` (`GetTransaction`) also has no timeout wrapper.
`walletDispatcher` holds per-wallet lock for entire execute goroutine (confirms: map stores action before
goroutine launch at line 220; removes only after goroutine returns at defer line 250). Unbounded RPC
blocking holds lock indefinitely. Severity Medium held.

### G1-F12: Re-anchor target search picks an untrusted wallet

Verdict: **Plausible** · Severity: **Medium** · Origin: **Introduced by PR #4282**

`reanchorTargetSearch.pick` (`pkg/tbtcpg/reservation_reanchor.go:927`) iterates over candidate Live wallets
and returns the first one with available reservation capacity, with no custody gate. A freshly registered
attacker wallet that calls `requestReservationReanchor` to set itself as the target will appear as a
candidate and will be selected if no other Live wallet has capacity. An attacker can register a wallet,
call `requestReservationReanchor` to direct a retiring wallet's reserved anchors to it, and have the
re-anchor task select their wallet as the target without any trust assessment.

The on-chain validator only checks that the target wallet state is `Live`; it does not apply any
custodian-trust criteria. The follower's co-signing path trusts the on-chain action record (which contains
the target). The attacker's wallet now custodies the depositor's tBTC collateral. The residual harm is the
depositor's recovery being tied to the attacker's wallet until that wallet is terminated or moves funds via
the moving-funds path.

The root cause is shared with G1-F2 (no custody gate on the re-anchor target). G1-F2 addresses the resume
path; this finding addresses the new-request target search. The code comment
(reservation_reanchor.go:838-852) acknowledges the "custody-scatter-not-theft tradeoff": scattering custody
to any Live wallet is by design, not a fund-safety failure. The remaining risk is recovery-path delay for
the depositor if the attacker wallet is slow or adversarial.

- Attack path: attacker registers a Live wallet, calls `requestReservationReanchor` targeting their
  wallet, and has the re-anchor task select their wallet as the target when no other Live wallet has
  capacity.
- Real fix: add a custody gate to `reanchorTargetSearch.pick`. Reject candidates that were registered too
  recently (e.g., within the last `minWalletAgeBlocks` blocks or a configurable gate), and/or require
  candidates to satisfy an operator-configured allowlist of trusted wallets. This is the same gate needed
  for G1-F2. Decision 2026-10-05: use the on-chain age gate (CreatedAt + walletCreationPeriod <= chain head
  time); no allowlist.
- Fix update (2026-10-05): the `pick` check is a leader-side courtesy. The follower check in
  `reservationReanchorAction.execute` (G1-F2) is the control that stops a malicious leader. See
  `recommendations-pr-4282.md` P3.
- Example real fix diff:
```diff
 // In reanchorTargetSearch.pick:
 candidate, err := rat.chain.GetWallet(candidates[i])
 if err != nil {
     continue
 }
+
+ registeredAt, err := rat.chain.GetWalletRegisteredAt(candidate.PublicKeyHash)
+ if err == nil && !rat.isTargetWalletTrusted(candidate.PublicKeyHash, registeredAt) {
+     taskLogger.Debugf("wallet [0x%x] does not satisfy custody gate, skipping", candidate.PublicKeyHash)
+     s.excluded[candidate] = true
+     continue
+ }
+
 if candidate.State != WalletActive { ... }
```
Second check: Plausible, held. Root cause shared with G1-F2. `reanchorTargetSearch.pick`
(reservation_reanchor.go:927) returns first Live wallet with available capacity, no trust gate. Freshly
registered attacker wallet can be selected as re-anchor target without any trust assessment. Same custody
gate needed as G1-F2.

## Verification Notes

- All findings were settled by static reading of the Go source (`pkg/tbtc/`, `pkg/tbtcpg/`), the x-ray
  invariant map ([rerun/x-ray-go/invariants.md](./rerun/x-ray-go/invariants.md)), and the pinned on-chain
  guards (`tbtc-v2@eec999aad:solidity/contracts/`).
- The "an on-chain check that stops the attack makes it Disputed" rule was applied to G1-F1
  (`WalletProposalValidator.validateDepositSweepProposal` `!isReservedDeposit`) and G1-F4 / G1-F5
  (`ReservationProofs` settlement dust floor + request-time invariant).
- G1-F5 and G1-F9 share the title and function `tbtc.AssembleReservationReanchorTransaction`; G1-F9 is
  ruled a **Duplicate** of G1-F5.
- G1-F2's suggested fix in the report is comment-only; a real fix is written in the section above.
- No source code was modified (read-only). No fix has been applied to the codebase in this pass.

## Supplementary run (lost agents)

The pass-3 first-principles agent died in the main scan; this single-agent supplement replaced it over the
same 11 in-scope files and the same scan bundle, outside the 5-pass loop (the ledger and the full report
were not modified by it).

## Counts

- New items triaged: **3** (S1, S2 Plausible Low; S3 Confirmed Low).
- Duplicates of existing `go-signing.md` items: **16** (mapped below).

Note: the triaged S rows are in the main Summary table above (this section keeps the per-item detail).

## New items

### G1-S1: Re-anchor task gates signing windows on node wall clock, not on-chain timestamps

Verdict: **Plausible (low-confidence LEAD)** - holds as an availability trail, not a confirmed exploit.

`ReservationReanchorTask.Run` (`pkg/tbtcpg/reservation_reanchor.go:288` and `:344`) compares
`uint64(time.Now().Unix())` against `action.TimeoutAt` and `reservation.ReanchorCooldownUntil`. Both are
Ethereum block timestamps (`TimeoutAt = uint32(block.timestamp)+...` at Reservation.sol:555/:778;
`reanchorCooldownUntil = uint32(block.timestamp)+...` at Reservation.sol:905-906), while the node reads its
host wall clock. The on-chain validator makes the same decision from `block.timestamp`
(WalletProposalValidator.sol:1001,:1132 `block.timestamp < action.timeoutAt - margin`).

Consequence is operator-side, not attacker-side: a fast host clock makes the node skip a generation the
on-chain validator would still accept (a bounded liveness miss, recovered on the next window); a slow host
clock makes it propose a generation the on-chain validator rejects, so the transaction must revert and the
operator's ETH is spent on a guaranteed revert. There is no unprivileged actor who controls the host
clock, so this fails Gate 3 (unprivileged trigger) and is not a confirmed fund-safety bug. It is a
hardening lead: sync the two timeout/cooldown checks against the latest on-chain block timestamp.

- Evidence: `pkg/tbtcpg/reservation_reanchor.go:288` (`uint64(time.Now().Unix())+... >= uint64(action.TimeoutAt)`),
  `:344` (`uint64(time.Now().Unix()) < uint64(reservation.ReanchorCooldownUntil)`); on-chain:
  Reservation.sol:555/:905-906, WalletProposalValidator.sol:1001/:1132.
- Origin: file is new in the PR (`git diff a7ac8989 49c4df13d` = 1366 insertions, 0 deletions).
- Fix: in `Run`, read the latest `HeaderByNumber` chain timestamp (the codebase already uses a bounded 30s
  `HeaderByNumber` read elsewhere) and compare `timeoutAt`/`reanchorCooldownUntil` against that instead of
  `time.Now()`.
- Prior knowledge: x-ray invariant I-11 (wall-clock vs chain-time, `Enforced=No`) names this class but only
  for the G2 SPV watcher files (`reservation_action_timeout_watch.go`, `reservation_stale_deposit_watch.go`).
  Related but distinct from G1-L28 (a re-anchor cooldown *race* that reverts the request and burns gas;
  G1-S1 is the deadline-gating clock source, not the race).

### G1-S2: Acceptance task gates its timeout checks on node wall clock, not on-chain timestamps

Verdict: **Plausible (low-confidence LEAD)** - holds as an availability trail, not a confirmed exploit.

`ReservationAcceptanceTask.findReservationAcceptanceCandidates` (`pkg/tbtcpg/reservation_acceptance.go:453`
and `:529`) compares `uint64(now.Unix())` (where `now := time.Now()` at :433) against
`requestEvent.TimeoutAt` and `action.TimeoutAt`, both Ethereum block timestamps set at request time
(Reservation.sol:555 `timeoutAt = uint32(block.timestamp)+...`). The on-chain acceptance validator gates the
same generation from `block.timestamp` (WalletProposalValidator.sol:1001-1004). A skewed host clock drops a
generation the validator would still sign (liveness miss, next window) or keeps one whose window the
validator considers closed (revert + operator ETH). No unprivileged actor controls the host clock, so Gate
3 fails; hardening lead, not a confirmed finding.

- Evidence: `pkg/tbtcpg/reservation_acceptance.go:433` (`now := time.Now()`), `:453` and `:529`
  (`uint64(now.Unix())+reservationRequestTimeoutSafetyMarginSeconds >= uint64(...TimeoutAt)`); on-chain:
  Reservation.sol:555, WalletProposalValidator.sol:1001-1004.
- Origin: file is new in the PR (1233 insertions, 0 deletions).
- Fix: same as G1-S1 - gate both timeout checks on the latest on-chain block timestamp read via the bounded
  `HeaderByNumber` call, not `time.Now()`.
- Prior knowledge: same class as G1-S1 / x-ray I-11. Distinct function, distinct timestamps; kept separate
  per the function-isolation rule.

### G1-S3: Follower fee-floor warning fires on every honest low-fee proposal

Verdict: **Confirmed** · Severity: **Low** · Origin: **Introduced by PR #4282**

The follower check `warnIfProposedWalletTxFeeBelowBufferedFloor` (`pkg/tbtc/proposal_fee_check.go`, new in
the PR) compares the proposed fee against the buffered floor
`ceil(MinWalletTxSatPerVByteFee * (100 + buffer) / 100) * vsize`. The leader-side
`tbtcpg.applyWalletTxFeeFloor` (`pkg/tbtcpg/fee.go`, formula identical to merge base `a7ac8989` with the
hard-coded `minWalletTxSatPerVByteFee = 5` at fee.go:37) emits
`max(ceil(estRate * (100 + buffer) / 100), floor) * vsize`, clamped down to the governance cap. With the PR
defaults (floor 5, buffer 25) an honest leader emits 5 sat/vB for mempool estimates of 4 sat/vB or less,
while the follower's buffered floor is 7 sat/vB, so the warning fires on every honest quiet-mempool
proposal.

- Evidence check: `evidence/g1/fee_floor_leader_vs_follower_test.go` (throwaway table test calling the real
  `tbtcpg.applyWalletTxFeeFloor`) and its output `evidence/g1/fee_floor_leader_vs_follower.out`. Two rows:
  `floor=5 buf=25 est=1 leaderRate=5 followerBufferedFloor=7 belowBuffered=true belowBare=false` and
  `floor=10 buf=25 est=9 leaderRate=12 followerBufferedFloor=13 belowBuffered=true belowBare=false`. In
  every row `belowBare=false`: no honest leader output lands below the bare floor.
- Fix: compare the warning against the bare floor (`MinWalletTxSatPerVByteFee * vsize`, no buffer - see
  recommendations P2 (D8)) and reject hard below the fixed protocol minimum. This removes the false alarm while keeping the degraded-fee signal.

## Duplicates of existing go-signing.md items

The agent was instructed to re-report listed bugs in full; 16 of the 18 items re-encounter items already in
`go-signing.md` (or its report/ledger). Mapped here; no new triage.

| Raw key (bug_class) | Existing G1 ID(s) |
|---|---|
| `tbtc.assemblereservationreanchortransaction` `invariant-divergence` | G1-F5 (dup G1-F9) |
| `tbtc.reservationanchoraction.execute` `fee-below-relay-floor` | G1-F7 |
| `tbtc.reservationreanchoraction.execute` `fee-below-relay-floor` | G1-F8 |
| `tbtc.depositsweepaction.execute` `fee-below-relay-floor` | G1-F6 |
| `tbtc.assembledepositsweeptransaction` `output-value-underflow` | a separate pre-existing item, not published |
| `tbtc.reservationreanchoraction.execute` `source-anchor-hash-unchecked` | G1-L36 (lead, not published) |
| `tbtcpg.reservationacceptancetask.findreservationacceptancereveal` `reveal-lookback-term-mismatch` | G1-L17 (lead, not published) |
| `tbtcpg.reservationreanchortask.pick` `unsigned-overflow` | G1-L23 (lead, not published) |
| `tbtcpg.finddeposits` `confirmations-error-swallow` | G1-L11 / G1-L15 (leads, not published) |
| `tbtc.depositsweepproposal.unmarshal` `fee-positivity-unmarshal-asymmetry` | G1-L4 (lead, not published) |
| `tbtcpg.estimatedepositssweepfee` `fee-vsize-witness-assumption` | G1-L9 (lead, not published) |
| `tbtcpg.reservationreanchortask.notifymovingfundsbelowdustifeligi...` `unbounded-scan` | G1-L19 / G1-L20 (leads, not published) |
| `tbtcpg.reservationreanchortask.wallethasreservationhistory` `rpc-reliance` | G1-L31 / G1-L32 (leads, not published) |
| `tbtcpg.reservationacceptancetask.findreservationacceptancecandidates` `reveal-cache-writeback-race` | G1-L16 (lead, not published) |
| `tbtcpg.applywallettxfeefloor` `fee-floor-cap-clamp-asymmetry` | a separate pre-existing item, not published |
| `tbtc.walletaction.execute` `onchain-validation-no-timeout` | G1-F11 |

## Verification notes

- All 18 raw items were gated against the repo files and the pinned on-chain guards; the two new items
  (G1-S1/S2) are confirmed code-level (wall clock compared to chain timestamps) but held at low confidence
  because the clock-skew precondition is operator-controlled and unmeasured.
- The raw agent output cited bundle-relative line numbers; the repo lines above were re-confirmed by
  reading `pkg/tbtcpg/{reservation_acceptance,reservation_reanchor}.go` and
  `tbtc-v2@eec999aad:solidity/contracts/bridge/{Reservation,WalletProposalValidator}.sol` directly.
- No tracked source file was modified. The scan ledger and full report are unchanged by this supplement
  (it records agent output only; the ledger is written by the main loop, not by a supplementary
  single-agent run).

## ID mapping

| Old ID | New ID |
|---|---|
| G1F1 | G1-F1 |
| G1F2 | G1-F2 |
| G1F4 | G1-F4 |
| G1F5 | G1-F5 |
| G1F6 | G1-F6 |
| G1F7 | G1-F7 |
| G1F8 | G1-F8 |
| G1F9 | G1-F9 |
| G1F11 | G1-F11 |
| G1F12 | G1-F12 |
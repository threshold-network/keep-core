# UTXO Reservations (M1) - epic tracker

This is the live tracker for every milestone and open item across the reservation PRs. Update it in the same change that completes an item.

- History up to 2026-09-28 is in `docs/plans/m1-delivery.md` on branch `docs/reservations-spec`. This file replaces its status sections from that date on.
- Scope: M1 only (create-only vault). M2 work (reserved redemption, multi-term) is listed under "Related, out of scope".

Last updated: 2026-10-29.

## Where we are

All work that can run without the deployer key is done. The next step is the M1 code freeze at the two tracker heads, targeted at 2026-11-06 at the earliest; the external audit reviews those frozen heads and has not started. The Sepolia deploy, which the key holder runs (milestone M4), and the testnet drill (M5) run inside the audit window, in parallel with it. Every fix in this epic is already public (M6 is down to the `dev` reconciliation).

```mermaid
flowchart LR
  M0[M0 review fixes merged] --> M1[M1 drill blockers]
  M1 --> M2[M2 tests and fork dry-run]
  M2 --> M3[M3 Sepolia deploy script]
  M3 --> M4[M4 Sepolia deploy]
  M4 --> M5[M5 testnet drill]
  M5 --> M6[M6 reconcile with dev]
  M6 --> M7[M7 mainnet readiness]
```

## Related PRs

| Repo | PR | Role | State (2026-10-29) |
|---|---|---|---|
| keep-core | #4282 `reservations-epic` -> `dev` | client tracker | open, draft, carries `dev`, 0 behind |
| keep-core | #4324, #4343, #4346 | review-fix rounds | merged into `reservations-epic` |
| keep-core | #4238 | earlier attempt | superseded by #4282, kept for reference |
| tbtc-v2 | #1116 `reservations-upgrade` -> `dev` | contracts tracker | open, draft, 0 behind |
| tbtc-v2 | #1161 | cross-repo review fixes | merged into `reservations-upgrade` |
| keep-core | #4366 `carry/reservations-spec-public-v5` | M1 spec | open, draft, into `reservations-epic` |

Related, out of scope for M1: tbtc-v2 #1129 (M2 reserved redemption), #1162 (multi-term), #1088 (superseded pre-M1 stack root).

## Milestones

### M0 - review-fix rounds merged - DONE

- [x] keep-core #4324, #4343, #4346 merged into `reservations-epic`.
- [x] tbtc-v2 #1161 merged into `reservations-upgrade`.

### M1 - drill blockers - DONE; external audit - NOT STARTED

- [x] Internal audit of #4282 and the matching tbtc-v2 contracts, published as a record in `docs/security/pr-4282-audit/` (the re-run under `rerun/`). This is an internal review of the branch, not an external audit and not an audit verdict.
- [ ] External audit of the frozen heads. Not started: the quote request went out on 2026-10-29 and no auditor has replied yet. It runs on the M1 code freeze heads.
- [x] Reservation findings fixed for this epic.
- [x] 7 drill-blocking reservation fixes made, each with a test that fails before the fix.
- [x] Third-party re-anchor hold cap per target wallet (tbtc-v2), with the client sending the member proof it needs.
- [x] Bridge guard: every reservation and deposit parameter setter refuses a value set that makes reserved deposits impossible (`now + revealAhead <= refundDeadline <= now + term + 24h`, and amount at least dust).
- [x] Client gauge `reservation_params_infeasible` warns when the live parameters fail the same check.
- [x] Spec updated for the hold cap and the feasibility guard (`docs/reservations-spec`).

Every fix in this milestone is public: it is on this branch and on tbtc-v2 #1116, landed 2026-10-28. There is no private review line left to publish.

### M2 - tests and fork dry-runs - DONE

- [x] Chained test: acceptance followed by re-anchor.
- [x] tbtc-v2 #1116 line merged with `dev`. All fixture suites pass locally (about 540 tests in 16 suites).
- [x] Mainnet-fork activation dry-run: 22/22.
- [x] Mainnet-fork lifecycle (reveal, acceptance request, acceptance proof): 25/25.
- [x] Lint rule that bans the self-catching `expect.fail` inside `try` pattern, with a policy test.

### M3 - Sepolia deploy script - DONE

- [x] Runbook reordered so that the client with the activation block ships before `setVaultStatus`.
- [x] One script with three modes: fork rehearsal, `PHASE=prepare` (steps 1-6 and 10, resumable), and `PHASE=activate ACTIVATION_BLOCK=<B>` (steps 11-12).
- [x] Live mode refuses a signer other than the deployer EOA, and refuses to activate before block B.
- [x] Rehearsed on a Sepolia fork (23 pass, 3 off-chain skips). Rehearsed live against a local Sepolia-fork node, with a killed and resumed prepare and real-time governance waits.

### M3b - depositor safety before the Sepolia deploy - DONE

- [x] Release to pool: the depositor can turn a reserved deposit that never got an acceptance request back into an ordinary deposit, which the next sweep picks up. This removes the long lockup for the loser of a capacity race (reveal does not hold capacity). Refused once any acceptance was requested, and never to the reservation vault.
- [x] keep-core bindings for the release, plus regression tests that a released deposit is swept with its stored vault.
- [x] SDK reservation module: pre-funding go/no-go check, wallet with free capacity, refund locktime inside the on-chain window with a reveal deadline, reveal, acceptance preflight and request, release, status reads.
- [x] Reservation proof and validator test suites repaired; they had not run since the ethers v6 migration.
- [x] Fork dry-runs and the Sepolia rehearsal re-run on the new contracts: 22/22, 25/25, 23 pass / 3 off-chain skips.

### M3c - hand-off capacity before the Sepolia deploy - DONE

- [x] A privileged hand-off off a retiring wallet (governance, the owner, an SPV maintainer, or a proven source-wallet member) may exceed the target wallet's per-wallet count and amount caps. Third parties keep both caps and the hold limit. This removes the case where full target wallets stop a retiring wallet from moving its reservations.
- [x] New governance value `handoffReserveWallets` (at least 1): an acceptance is admitted only while `count + 1 <= (Live wallets - reserve) * per-wallet count`. It limits admission; it does not reserve a specific wallet.
- [x] The proposal validator no longer re-checks target caps that the request already booked. The client falls back to the least-loaded Live wallet for a member hand-off when every target is full. The SDK checks the reserve before funding and before acceptance.
- [x] Caps setter now takes five arguments; new view `reservationHandoffReserveWallets()`. Fork dry-run 34/34 and Sepolia rehearsal re-run on the new contracts.

### M4 - Sepolia deploy - PENDING, inside the audit window (waits on the key holder)

- [ ] Recommended, waiting for approval: before resuming, validate the saved prepare file. Tie it to the chain ID and the Bridge address, and require chain ID 11155111 on `--network sepolia`.
- [ ] Key holder runs `PHASE=prepare` on Sepolia and shares the printed addresses.
- [ ] Publish npm `@keep-network/tbtc-v2@sepolia` with the reservation artifacts.
- [ ] Set the Sepolia activation block B in `pkg/tbtc/coordination.go`, regenerate the bindings, build the testnet client.
- [ ] Drill operators (existing Sepolia operators) run that build.
- [ ] Key holder runs `PHASE=activate ACTIVATION_BLOCK=<B>` at or after block B.
- [ ] Read-only check of the deployed state against the runbook values.

### M5 - testnet drill - TODO

- [ ] Fill in the remaining TBD items in the drill runbook (addresses, block B, operator list).
- [ ] Run the drill: reserved deposit, acceptance, re-anchor, timeout and strand paths, plus alarms.
- [ ] Alert on acceptance generations that are still unproven as they near the end of their late-settlement window (`timeoutAt + termSeconds`), plus a governance escalation runbook. Build it before the drill and exercise it during the drill.
- [ ] Record the results and any new findings.

### M6 - reconcile with dev - TODO

- [x] Open the reservation-only fixes as public PRs into `reservations-epic` and `reservations-upgrade`. Landed 2026-10-28 on the client side and in the tbtc-v2 line; nothing was held back for publication.
- [x] Resolve the conflicts of #4282 and #1116 with `dev`. Both carry `dev` at their heads: 0 commits behind, `mergeable: MERGEABLE` and a clean merge state as of 2026-10-29.
- [x] Open a PR for the spec branch: keep-core #4366 from `carry/reservations-spec-public-v5`, draft, into `reservations-epic`.
- [ ] Merge both trackers into `dev`, after the external audit's findings are fixed and re-reviewed.

### M7 - mainnet readiness - TODO

- [x] Third-party re-anchor hold limit: closed by the privileged hand-off in M3c.
- [ ] Choose launch values that pass the feasibility guard. Amount caps must be at least dust (0.01 BTC); a 12-month term is fine.
- [ ] Keep a Bridge bytecode budget: 1,009 bytes are left under EIP-170.
- [x] SDK pre-funding check (done in M3b): go or no-go on the refund-locktime window, the amount bounds including dust, the designated wallet being `Live` with free capacity, and the global caps.
- [ ] Mainnet runbook through Timelock and Council Safe, rehearsed on a mainnet fork.

## Open items not tied to a milestone

- [ ] Fork lifecycle step L3: also assert the depositor's TBTC balance change (and the fee recipient's change, if a fee is charged), not only `mintedAmount`.

## Decisions

| Date | Decision |
|---|---|
| 2026-10-10 | The fork lifecycle covers reveal and acceptance only. Unit tests and the drill cover re-anchor, timeout and strand. |
| 2026-10-10 | Accept the known third-party hold limit for testnet; decide a fix before mainnet. |
| 2026-10-10 | Script and rehearse the Sepolia runbook. The key holder runs it; the fee reserve target is 0. |
| 2026-10-10 | The drill uses the existing Sepolia operators. |
| 2026-10-10 | Publish the fixes after the drill. Superseded 2026-10-28. |
| 2026-10-10 | Approved: Bridge feasibility guard, lint ban, client feasibility gauge. Declined: a nightly fork dry-run in CI. |
| 2026-10-12 | Add release to pool and the SDK reservation module before the Sepolia deploy. |
| 2026-10-12 | Privileged hand-offs off retiring wallets skip per-wallet caps, and a hand-off reserve gates new acceptances. Built before the Sepolia deploy. |
| 2026-10-28 | The reservation fixes are public: they land on `reservations-epic` and on tbtc-v2 `reservations-upgrade` instead of waiting for the drill, so there is no separate publication step. |
| 2026-10-29 | The external audit runs on the M1 code freeze heads; the Sepolia deploy and the testnet drill run inside the audit window rather than before it. |

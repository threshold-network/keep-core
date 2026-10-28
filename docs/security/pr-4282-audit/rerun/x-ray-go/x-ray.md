# X-Ray Report

> keep-core - tBTC v2 UTXO Reservations Epic (PR #4282, merged head) | 64,468 nSLOC (17,960 in-scope hand-written) | `9f99f364a` (detached, the integrated branch + the audit-fix branch) | Go off-chain client | 06/10/26

Analyzed head: `9f99f364a28a7dd361f1942b635476bea723a8af`. This is the final merged head of the reservations epic plus the audit-fix branch (fixes for 36 findings and 13 leads) merged with `origin/dev` (`d72bca6fc`). Scope: the 73 non-test Go files in the Phase C re-run scope list (an internal run record, not part of this record) (50 hand-written files, 17,960 nSLOC in 8 subsystems; 23 generated ABI-binding files, 46,508 nSLOC, listed for reference but out of analysis scope). The PR diff is `git diff origin/dev 9f99f364a` (73 files, +33,378 / -204).

**Adaptation note.** This skill targets Solidity protocols; here it is applied to the Go off-chain client. Mapping used throughout: contract -> component, on-chain guard -> Go validation guard, "On-chain" enforcement -> `Enforced` (Yes/No in `invariants.md`), Solidity entry-point grep gate -> Go grep gate (see `entry-points.md`), forge coverage -> `go test -cover`. The on-chain reservation contracts (`ReservationRouter`/`ReservationVault`/`Bridge`/`WalletProposalValidator`, tbtc-v2) are out of scope and treated as external trust boundaries; spec claims tagged `(per spec)` come from `docs/spec/reservations/**` on branch `origin/docs/reservations-spec`, which is not merged into this head.

**Phase C re-run note.** This is a re-run of the first audit's Go x-ray (verdict EXPOSED, 75 G / 17 I / 8 X / 4 E, single Enforced=No invariant I-11 "wall clock vs chain time in all watcher deadline decisions"). The fix branch re-pointed every reservation deadline decision to the chain head's block time (`pkg/chain/ethereum/tbtc_chain_time.go`, `TbtcChain.GetLatestBlockTimestamp`, 24 lines, new). The previous I-11 now holds for all reservation lifecycle paths; the wall-clock-vs-chain-time gap survives at two non-reservation sites (I-11, I-18 below). The delta against the first run is in [../xray-go.md](../xray-go.md).

---

## 1. Protocol Overview

**What it does:** the off-chain signer/maintainer client that custody-anchors tBTC v2 UTXO reservations - generating 1-in-1-out anchor/re-anchor Bitcoin proposals, threshold-signing them, and proving settlement to the on-chain Bridge.

- **Users**: keep-core wallet signing groups, SPV maintainer operators, tBTC depositors requesting reservation acceptance
- **Core flow**: coordinator proposes a reservation anchor/re-anchor; followers simulate against `WalletProposalValidator` (plus the new follower-side re-anchor target-age gate), threshold-sign a 1-in-1-out P2WPKH transaction; SPV maintainer submits the proof when confirmed
- **Key mechanism**: UTXO reservation lifecycle (Pending action -> Settled/TimedOut) with 2-hour signing margins, 300-block safety margins, and permissionless housekeeping watchers that now judge every deadline on chain head time (per spec, per fix `ad0575e0e`/`768707517`)
- **Token model**: tBTC minted to `ReservationVault` at acceptance (per spec); in-kind fee debt (`inKindFeeDebtSat`) tracked on-chain
- **Admin model**: operator CLI/config (activation, fee floor/buffer, watchers); on-chain governance (Threshold Council Safe, 48h timelock per spec) sets reservation parameters

The Phase C architecture diagram was not retained in the committed evidence snapshot.

### Components in Scope

| Subsystem | Key Components | nSLOC | Role |
|-----------|--------------|------:|------|
| `pkg/tbtc` | reservation.go, reservation_target_age.go, coordination.go, deposit_sweep.go, marshaling.go, proposal_fee_check.go, moving_funds.go, redemption.go | 6,032 | Follower actions, wallet coordination, on-chain proposal validation, fee policy, wire (protobuf) marshaling |
| `pkg/tbtcpg` | reservation_acceptance.go, reservation_reanchor.go, fee.go, deposit_sweep.go, moving_funds.go | 4,378 | Coordinator proposal generators (acceptance, re-anchor, sweep, redemption, moving funds) and fee floor/buffer math |
| `pkg/maintainer/spv` | reservation_proof_loop.go, reservation_action_timeout_watch.go, reservation_stale_deposit_watch.go, reservation_wiring.go | 3,434 | SPV proof submission loops and housekeeping watchers (timeout, stale deposit, stranding) |
| `pkg/chain/ethereum` | tbtc.go, ethereum.go, tbtc_chain_time.go | 1,772 | Ethereum adapter: views, event logs, tx submission (20% gas margin), chain head time accessor, ABI bindings |
| `cmd + clientinfo` | start.go, maintainer.go, maintainercli.go, performance.go | 1,431 | CLI wiring, operator flags (fee floor/buffer, monitor), paired reservation-flag deploy check, Prometheus metrics |
| `net / tecdsa / beacon` | broadcast channel/session isolation, tecdsa DKG+signing marshaling, beacon DKG marshaling | 903 | P2P broadcast (session-isolated), tECDSA wire format, beacon key-share range check |
| **TOTAL IN-SCOPE** | 50 hand-written files | **17,960** | PR-touched nSLOC vs origin/dev |
| `pkg/chain/ethereum/tbtc/gen/**` (out of scope) | 23 generated binding files | 46,508 | abigen output for Bridge, LightRelay, ReservationRouter/Vault, WalletProposalValidator |

Backwards-compatibility subsection omitted: no compatibility remnants survived into scope; `tbtc_inactivity.go`/`tbtc_sortition.go` remain pure moves from `tbtc.go` with zero semantic change (verified against merge-base), which is a move, not a remnant.

### How It Fits Together

The core trick: reserved deposits are custody-anchored by a strict 1-in-1-out P2WPKH UTXO lineage, and every lifecycle transition (accept, re-anchor, timeout, strand, stale-release) is closed by an on-chain SPV proof or permissionless notification that the client itself must trigger. Since the audit-fix branch, every time-based decision in that machinery reads the Ethereum head's block time instead of the host clock.

#### Reservation Acceptance

```
depositor: requestReservationAcceptance (on-chain Bridge)
  └─ ReservationAcceptanceTask.Run (tbtcpg)
       ├─ candidate filter ── chainHead+7200s < TimeoutAt, deposit mature (chain time), >=6 BTC confirmations
       ├─ fee estimate ───── floor 5 sat/vB + 25% buffer, clamped to TxMaxFee
       ├─ ValidateReservationAnchorProposal  ◄── on-chain view against WalletProposalValidator
       └─ broadcast → reservationAnchorAction.execute (followers)
            ├─ re-check on-chain action pending + target wallet
            ├─ AssembleReservationAnchorTransaction (1-in-1-out, fee ≤ TxMaxFee)
            ├─ signTransaction (tECDSA threshold; 300-block margin)
            └─ broadcastTransaction             ◄── BTC anchor UTXO created
                 └─ SPV proof loop → SubmitReservationAcceptanceProof  ◄── tBTC minted to vault
```

#### Re-Anchor & MovingFunds Wind-Down

```
wallet MovingFunds/Closing holding reservations
  └─ ReservationReanchorTask.Run (tbtcpg)
       ├─ target search ── Live wallets, count/amount headroom, cooldown elapsed,
       │                   target older than creation period (IsReanchorTargetAged, chain time)
       ├─ RequestReservationReanchor (at most 1 on-chain tx per pass)
       └─ reservationReanchorAction.execute → follower re-checks target age
            ◄── GetLatestBlockTimestamp + IsReanchorTargetAged(…, ReanchorTargetAgeTolerance)
            └─ SPV proof → custody shifted; fee financed by vault (inKindFeeDebtSat)
                 └─ 0 reservations left + main UTXO < dust → NotifyMovingFundsBelowDust
```

#### Housekeeping Watchers (SPV maintainer)

```
pending action unproven ─── chain head >= TimeoutAt (+ operator stagger, 10-min renotify backoff)
  ├─ alertOverdueAction (Warn + spv_reservation_action_overdue_total metric, NEW)
  ├─ NotifyReservationAcceptanceTimedOut      ◄── releases capacity; late settlement still allowed
  └─ NotifyReservationActionTimeout → stranding recheck queue
stale deposit ── chain head > refundDeadline and action not Pending → NotifyStaleReservedDeposit
wallet Closed/Terminated → checkReservationStrandingForWallet → NotifyReservationStranded
                          (Closing form additionally waits for DissolutionEligibleAt in chain time)
```

#### Coordination Window

```
coordination window (chain block-driven)
  ├─ leader: ProposalGenerator.Generate ── checklist adds ActionReservationAnchor/Reanchor
  │                                               ◄── iff coordinationBlock ≥ ReservationsActivationBlock(network)
  └─ follower: validateMessage ── membership, block, wallet PKH, leader identity,
                                               allowed-action list (fault attribution)
       └─ handleReservationAnchor/ReanchorProposal → walletDispatcher (one action per wallet)
```

---

## 2. Threat & Trust Model

### Protocol Threat Profile

> Component classified as: **Bridge/Custody off-chain threshold-signing client** with **state-machine lifecycle** characteristics

The code signs and proves movement of custodied BTC value (bridge + custody profile), and every reservation state transition (Pending -> Settled/TimedOut -> Stranded/Closed) is a discrete, irreversible lifecycle the client must keep honest. Adversary lists below merge: bridge/custody signers and keepers first, then lifecycle/state-machine threats (stale generations, deadline races, now on chain time), de-duplicating overlapping ones.

### Actors & Adversary Model

| Actor | Trust Level | Capabilities |
|-------|-------------|-------------|
| Depositor / reservation owner | Bounded (off-chain, no on-chain authority over the wallet) | Reveals deposits to the reservation vault, requests acceptance, permissionless fee-debt repayment (per spec) |
| On-chain keeper / watcher (any caller) | Bounded (permissionless, rewardless; contract state gates effects) | Triggers `notify*` housekeeping; early/late calls revert, idempotent no-ops (per spec) |
| SPV maintainer (`onlySpvMaintainer`) | Trusted (operator-selected) | Submits acceptance/re-anchor SPV proofs; wrong proof burns gas, forgery impossible on-chain |
| Coordination leader (per-window, deterministic) | Bounded (follower view-simulation + quorum) | Proposes fees/targets; impersonation or off-checklist action -> `FaultLeaderImpersonation`/`FaultLeaderMistake` |
| Wallet signing group (tECDSA quorum) | Bounded (threshold quorum + Bridge fraud challenge per spec) | Threshold-signs 1-in-1-out txs; unauthorized signing exposed to fraud challenge/slashing |
| Operator (node owner) | Trusted | Sets activation, fee floor/buffer, watcher intervals; host clock now used ONLY for renotify backoff and `notifiedAt` stamps - deadline decisions are on chain head time |
| Electrum server | Bounded (untrusted data source) | Serves tx bodies, confirmations, fee estimates, SPV block headers |
| Ethereum RPC provider | Bounded (untrusted data source) | Serves views, `eth_getLogs`, gas estimates; a lagging or reorging head time defers (never advances) watcher notifications |

**Adversary Ranking** (ordered by threat level for this component type, adjusted by git evidence):

1. **Malicious minority of signers** - an adversarial quorum-minus-one can still propose or co-sign; the Go client's pre-signing checks (on-chain action record + `WalletProposalValidator` view + the new target-age gate) are the last off-chain line before threshold signature.
2. **Malicious libp2p peer** - untrusted protobuf payloads and forged coordinator identities enter the signing path; malformed input must be rejected, not crash or stall honest signers. The ephemeral-key map and beacon private-key-share unmarshalers now reject out-of-range values at unmarshal time (`eb8c3d39f`, `bc1a0aa10`).
3. **Malicious coordination leader** - proposes bad fees, wrong targets, or off-checklist actions; the follower now hard-rejects any proposed fee below the 5 sat/vB protocol minimum on every wallet-tx path (`rejectIfProposedWalletTxFeeBelowProtocolMin`, `30dac8c0f`).
4. **Malicious on-chain keeper** - can race the client's watchers on housekeeping notifications; client paths assume idempotency and re-verify before submit.
5. **Unreliable infrastructure (Electrum/RPC)** - stale or adversarial data feeds proposal generation, fee estimation, and SPV proof assembly; a slow/unreadable chain head now simply defers deadline decisions rather than firing them early.

**Permissionless entry points** (grep-verified; full detail in [entry-points.md](entry-points.md#permissionless)):

- `spv.ReservationActionTimeoutWatcher.Run()`
- `spv.ReservationActionTimeoutWatcher.CheckReservationActionTimeouts()`
- `spv.ReservationStaleDepositWatcher.Run()`
- `spv.ReservationStaleDepositWatcher.CheckStaleReservedDeposit()`
- `spv.reservationStrandingWatcher.checkReservationStrandingForWallet()`
- `spv.subscribeReservationWalletClosed` (event handler)
- `ethereum.TbtcChain.OnReservationAcceptanceRequested()`
- `ethereum.TbtcChain.OnReservationReanchorRequested()`
- `ethereum.TbtcChain.PastReservationAcceptanceRequestedEvents()`
- `ethereum.TbtcChain.PastReservationReanchorRequestedEvents()`
- `ethereum.TbtcChain.NotifyMovingFundsBelowDust()`
- `ethereum.TbtcChain.WalletReservations()`
- `ethereum.TbtcChain.WalletTerminationCause()`
- `ethereum.TbtcChain.GetLatestBlockTimestamp()` (NEW - head-time accessor feeding all deadline gates)
- `tbtc.FindDepositRevealedEvent()`
- `tbtc.FindDepositRevealedEventByRevealTime()`
- `tbtc.unmarshalCoordinationProposal()`
- `tbtc.ReservationAnchorProposal.Unmarshal()`
- `tbtc.ReservationReanchorProposal.Unmarshal()`
- `clientinfo.Initialize` (HTTP metrics endpoint; pprof endpoints REMOVED)
- `crypto.UnmarshalPublicKey()`

### Trust Boundaries

- **Peer -> signer P2P boundary** - untrusted bytes; strict unmarshal bounds (`pkg/tbtc/marshaling.go:505-645`) + membership/leader checks (`pkg/tbtc/coordination.go:880,898`); ephemeral public-key maps (`pkg/tecdsa/dkg/marshaling.go`, `pkg/tecdsa/signing/marshaling.go`) and beacon private key shares (`pkg/beacon/dkg/marshaling.go:76`) reject out-of-range values at unmarshal; no time delay - a bad message is dropped per-message, not throttled.
- **Signer -> on-chain boundary** - every signature path gates on a pending on-chain action record and a `WalletProposalValidator` view call (`pkg/tbtc/reservation.go:671-692,852-870`), plus the target-age gate (`pkg/tbtc/reservation.go:901-916`) and the protocol fee minimum (`pkg/tbtc/reservation.go:713,950`); the on-chain contracts are the external trust boundary, out of scope.
- **Operator host -> chain-time boundary (FIXED)** - the previous single Enforced=No invariant is closed: every reservation deadline decision (timeout-notify gate, stale-deposit refund gate, late-window eviction, abandon gate, dissolve gate, acceptance candidate gate, re-anchor cooldown/margin/age gates) now reads the chain head's block time via `GetLatestBlockTimestamp` and skips the decision when the head is unreadable (`pkg/maintainer/spv/reservation_action_timeout_watch.go:461-465`, `reservation_stale_deposit_watch.go:272-281`, `reservation_proof_loop.go:494-498,1166-1172`, `reservation_stranding_watch.go:177,256-258`). Host wall clock remains at two non-reservation sites (I-11, I-18).
- **Client -> deployment config boundary** - the activation-block table is still empty at this head, so the feature is off on all public networks; double CLI opt-in (`Tbtc.ReservationsEnabled` + `Spv.ReservationProofsEnabled`), plus a new out-of-band deploy check that compares the two flags across the two config files (`scripts/check-reservation-flags.sh`, `7c8269d6a`).
- **Electrum/RPC -> proof boundary** - UTXO values and confirmations from Electrum feed on-chain submissions; SPV difficulty + Merkle verification happens on-chain, but fee estimates and target search read untrusted data. Registration-block lookups now fail the pass instead of degrading to genesis scans (`4ca44edad`).

### Key Attack Surfaces

- **Moving-funds safety margin on wall clock** &nbsp;&#91;[I-18](invariants.md#i-18)&#93; - `pkg/tbtc/moving_funds.go:479` compares host `time.Now()` to `MovingFundsRequestedAt + safetyMargin` (24 h / 14 d); a skewed host clock opens the margin early or defers it. The audit fixes re-pointed the reservation paths only; this pre-existing gate still rides the host clock.

- **Deposit-sweep generator maturity gate on wall clock** &nbsp;&#91;[I-19](invariants.md#i-19)&#93; - `pkg/tbtcpg/deposit_sweep.go:223,288` uses host `time.Now()` for the deposit minimum-age decision while the reservation tasks use chain time; asymmetric clock domains for two generators in the same checklist window.

- **Head-time unavailability fail-closed semantics** &nbsp;&#91;[I-11](invariants.md#i-11)&#93; - when `GetLatestBlockTimestamp` fails, `reservationChainHeadTime` returns the zero time and every reservation deadline/eviction decision is skipped for that pass (`pkg/maintainer/spv/reservation_proof_loop.go:494,1166`, `reservation_action_timeout_watch.go:461`, `reservation_stale_deposit_watch.go:273`); a chronically failing RPC provider silently stalls notifications, and the stranding Closing-form path instead hard-fails the pass (`reservation_stranding_watch.go:177-184`). Worth confirming the failure mode is observable.

- **Late-settlement and proof-loop bookkeeping** &nbsp;&#91;[G-59](invariants.md#g-59), [G-60](invariants.md#g-60), [I-16](invariants.md#i-16)&#93; - `reservation_proof_loop.go:494-498,528,939` evicts TimedOut generations only past `TimeoutAt+TermSeconds` on chain head time, abandons unnotified Pending generations at 4x the request window, and caps one proof per key per pass; the eviction predicates must track the on-chain term window exactly.

- **Stranding has four concurrent trigger paths plus a dissolution gate** &nbsp;[I-12](invariants.md#i-12), [X-7](invariants.md#x-7) - `checkReservationStrandingForWallet` (`reservation_stranding_watch.go:246`) is invoked from `reservation_wiring.go` and `reservation_action_timeout_watch.go:755` with no global lock; dedup rests on the Bridge no-op assumption; the Closing-wallet form now additionally gates on `DissolutionEligibleAt` in chain time (`reservation_stranding_watch.go:256-258`).

- **Re-anchor target age: three chain-time gates** &nbsp;&#91;[I-9](invariants.md#i-9), [E-3](invariants.md#e-3)&#93; - the generator target search (`pkg/tbtcpg/reservation_reanchor.go:368,1101`), the resume path, and the follower co-sign gate (`pkg/tbtc/reservation.go:901-916`) all require the target wallet to have aged out of its creation period, measured on the chain head with a 24 s tolerance (`pkg/tbtc/reservation_target_age.go:20-28`); worth checking the tolerance keeps leader and follower views from disagreeing at the cutoff.

- **Router binding via Bridge delegatecall address** &nbsp;[I-17](invariants.md#i-17) - `pkg/chain/ethereum/tbtc.go:341` constructs the `ReservationRouter` binding against the Bridge address; a call against the standalone router address would read empty storage.

- **Leader/follower checklist determinism** &nbsp;[X-6](invariants.md#x-6) - reservation actions join the checklist iff `coordinationBlock >= ReservationsActivationBlock(network)` (`pkg/tbtc/coordination.go:714`); a follower with mismatched network config misattributes `FaultLeaderMistake` (`pkg/tbtc/coordination.go:913`).

- **Protocol fee minimum: leader-hard / follower-hard** &nbsp;[I-7](invariants.md#i-7), [X-4](invariants.md#x-4) - the follower now hard-rejects any wallet-tx fee below 5 sat/vB x (vsize-1) on sweep/redemption/moving-funds/reservation paths (`pkg/tbtc/proposal_fee_check.go:183-204` called from `reservation.go:713,950`, `deposit_sweep.go:705`, `moving_funds.go:377`, `redemption.go:399`); the operator's own floor/buffer stays a soft warn. Worth checking mixed-version divergence still cannot stall signing quorums.

- **Peer-supplied proposal unmarshaling** &nbsp;[I-15](invariants.md#i-15) - `marshaling.go:505-645` rejects zero nonces, oversized fees/keys, and mismatched PKH/hash lengths before proposal structs are built; the ephemeral-key and beacon share unmarshalers reject out-of-range values. Worth confirming every network entry path reaches these guards.

- **Reserved-deposit sweep exclusion split across two components** &nbsp;[I-3](invariants.md#i-3), [X-1](invariants.md#x-1), [E-4](invariants.md#e-4) - the generator pre-filters (`pkg/tbtcpg/deposit_sweep.go:268-275`) and the follower hard-rejects (`pkg/tbtc/deposit_sweep.go:620`) via `chain.IsReservedDeposit`; worth confirming both read the same on-chain source and no third sweep path bypasses the check.

- **Registration-block memoization bounded lookups** &nbsp;[X-8](invariants.md#x-8) - `earliestWalletRegistrationBlock` failures now fail the wallet query instead of falling back to `startBlock = 0` (`pkg/chain/ethereum/tbtc.go:1390-1488`, `4ca44edad`); one unreadable reservation no longer aborts the whole wallet lookup. Worth checking the failure mode stays bounded.

- **Re-anchor request once-per-pass discipline** &nbsp;[I-8](invariants.md#i-8) - the post-submit failure sentinel (`pkg/tbtcpg/reservation_reanchor.go:504`) terminates the pass after the single `RequestReservationReanchor` submit.

- **Target PKH re-verification at proof submission** &nbsp;[X-3](invariants.md#x-3) - `reservation_reanchor_proof.go:255` re-reads the on-chain action and re-asserts `pkh == action.TargetWalletPublicKeyHash` immediately before submit.

- **300-block signing margin underflow guards** &nbsp;[I-14](invariants.md#i-14) - `reservation.go:742,979` and `deposit_sweep.go:324` reject expiries closer than 300 blocks to the start block.

### Upgrade & Configuration Concerns

- **Vendored ABI fallback artifacts** - `pkg/chain/ethereum/tbtc/gen/Makefile` copies placeholder `ReservationRouter`/`ReservationVault` JSON (zero address) and patches stale `Bridge`/`WalletProposalValidator` artifacts in development only; CI pins tbtc-v2 at `eec999aad`. Worth confirming the committed bindings still byte-match the shipped tbtc-v2 artifacts.

- **Empty activation-block table** - `reservationsActivationBlocks` (`pkg/tbtc/coordination.go:94`) is empty at this head; every non-Developer network returns `math.MaxUint64` (feature off). Worth confirming a real block is chosen before any public-network rollout.

- **Dual CLI opt-in flags + out-of-band deploy check** - `Tbtc.ReservationsEnabled` (`pkg/tbtc/tbtc.go`) and `Spv.ReservationProofsEnabled` must both be set; mispairing warns in-process (`cmd/start.go:235-241`, `reservation_wiring.go` self-check) and is now additionally cross-checked by `scripts/check-reservation-flags.sh` at deploy time (`7c8269d6a`).

- **pprof removed from the metrics server** - the `EnablePprof` setting and `/debug/pprof/*` registration are gone (`pkg/clientinfo/clientinfo.go`, `38f8673e4`); the metrics port serves `/metrics` only. The old surface that let an exposed clientinfo port leak runtime profiles is closed.

- **Gas margin math** - `ethereum.go` `GasEstimateWithMargin` now uses ceiling integer math (`(estimate*20+99)/100`, `4c9ace3ad`), but `tbtc.go` still carries 8 inline `float64(gasEstimate) * 1.2` sites (around L849/902/953/981/1006/1031/1170/1212) that were not migrated to the shared helper. Worth confirming the two margin implementations agree.

### Protocol-Type Concerns

**As a bridge/custody off-chain signer client:**
- The Go client's only hard reliance is the tECDSA threshold quorum: `signTransaction` (`pkg/tbtc/reservation.go:746,983`) aggregates the group signature; minority signers can delay but not block, and fault attribution (`coordination.go:898,913`) is the sole peer-side detection.
- On-chain governance changes reservation parameters/caps with a 48h timelock (per spec) while the client snapshots parameters per action generation; the re-anchor cooldown and caps are now read through `GetWalletParameters()` and the re-anchor target-age gate, so in-flight target choices can age out mid-proposal - the 24 s tolerance and the per-pass head-time snapshot are the mitigation.
- Fee-policy package vars (`MinWalletTxSatPerVByteFee`, `WalletTxFeeBufferPercent`, set once in `pkg/tbtc/tbtc.go` `applyWalletTxFeePolicy`) are read lock-free across tasks, with the new fixed protocol minimum `ProtocolMinWalletTxSatPerVByteFee = 5` as the hard floor (`tbtc.go`, `proposal_fee_check.go:191`).

**As a bridge SPV client (secondary):**
- Settlement window is bounded by `MaxProofHeaders = 144` (`pkg/maintainer/spv/spv.go:634`) and the DIFF1-header skip; a DIFF1-heavy tail can skip proofs until the relay epoch updates.
- Event scans trail the tip by 12 blocks (`pkg/maintainer/spv/reservation_event_scan.go:52-53`), a reorg buffer that shifts what "confirmed" means at the scan boundary; chunk errors preserve the cursor instead of re-scanning activation-to-tip.

### Temporal Risk Profile

**Deployment & Initialization:**
- Startup catch-up scans are bounded to a 216,000-block (30-day) lookback (`reservation_wiring.go:641`); a network that was inactive longer than that misses closed-wallet stranding until the event path fires.
- The misconfiguration self-check (`reservation_wiring.go:618`) only warns - a fully-quiet initial scan with an unpaired flag logs an `Errorf` and keeps running; the new deploy check (`scripts/check-reservation-flags.sh`) catches the mispairing out-of-band.

**Steady-State Timing:**
- Every reservation deadline decision is now chain-head-time based (I-11 holds for the reservation paths; I-18/I-19 are the two surviving wall-clock sites outside them); the 7200s proposal margin (I-6), 300-block signing margin (I-14), re-anchor cooldown (G-51), and 10-minute renotify backoff (G-63) ride on the re-pointed clock sources. Host wall clock survives at `moving_funds.go:479` and `pkg/tbtcpg/deposit_sweep.go:223,288`.
- Late settlement: a TimedOut acceptance stays provable until the chain head passes `TimeoutAt + TermSeconds` (`reservation_proof_loop.go:494`); abandoned-but-unnotified generations are evicted at `TimeoutAt + 4*(TimeoutAt-RequestedAt)` (`reservation_proof_loop.go:1166-1172`).

### Composability & Dependency Risks

**Dependency Risk Map:**

> **Electrum (Bitcoin)** - via `pkg/bitcoin/electrum`
> - Assumes: returned tx bodies, confirmation counts, and headers are correct and up to date
> - Validates: >=6 funding-tx confirmations before acceptance proposals (`reservation_acceptance.go:896`); SPV proofs verified on-chain; re-anchor source-anchor hash re-checked before late proof submission (`reservation_proof_loop.go:782`)
> - On failure: candidate skipped per pass; transaction monitor treats lookup errors as unconfirmed

> **Ethereum RPC** - via `pkg/chain/ethereum` TbtcChain views, `eth_getLogs`, gas estimates, `GetLatestBlockTimestamp`
> - Assumes: views reflect current state; provider supports 10,000-block log chunks; head timestamp is the latest header time
> - Validates: active health probe; ABI enum range checks; 20% gas margin (ceiling integer math in `GasEstimateWithMargin`, inline float math in `tbtc.go`)
> - On failure: a lagging head defers watcher notifications (fail-safe direction); unreadable head disables eviction for the pass; registration-block lookup failure fails the wallet query (`4ca44edad`)

> **libp2p peer messages** - via `pkg/tbtc/marshaling.go` unmarshalers and `pkg/tecdsa/{dkg,signing}`, `pkg/beacon/dkg`
> - Assumes: proposal/ephemeral-key/key-share bytes are well-formed
> - Validates: strict fee/nonce/key/PKH bounds (`marshaling.go:505-645`); compressed-point parse for every ephemeral key map entry (`eb8c3d39f`); bn256 private-key-share range `[1, order)` (`bc1a0aa10`); membership + leader-identity checks (`coordination.go:880,898`)
> - On failure: message rejected, sender marked inactive, or fault attributed

> **tbtc-v2 contracts (ABI)** - via generated bindings + vendored fallback JSON
> - Assumes: ABI stable at the pinned tbtc-v2 commit; committed Go bindings byte-match compiled artifacts
> - Validates: `checkabi` canonical-JSON comparison and `verify-vendored-fallback` CI job against pinned commit `eec999aad`
> - On failure: reverts with contract error reasons; the dominant drift risk is contract-side (out of scope)

**Token Assumptions** *(unvalidated only)*:
- tBTC `balanceOf` reads in `tbtc.go` are used for fee-reserve metrics only - no settlement logic depends on token behavior.

**Shared State Exposure:**
- Multiple independent SPV maintainer operators can submit proofs and `notify*` transactions against the same on-chain reservation state; dedup is one-proof-per-key-per-pass locally plus Bridge idempotency (per spec) - no client-side coordination between operators. The new per-operator first-notify stagger (`reservation_action_timeout_watch.go:434-447`) spreads first attempts across a 600 s window instead of one tick.

---

## 3. Invariants

> ### Full invariant map: [invariants.md](invariants.md)
>
> A dedicated reference file contains the complete invariant analysis - do not look here for the catalog.
>
> - **67 Enforced Guards** (`G-1` ... `G-67`) - per-call preconditions with `Check` / `Location` / `Purpose`
> - **19 Single-Component Invariants** (`I-1` ... `I-19`) - Conservation, Bound, Ratio, StateMachine, Temporal
> - **8 Cross-Component Invariants** (`X-1` ... `X-8`) - producer/consumer pairs across subsystem boundaries
> - **4 Economic Invariants** (`E-1` ... `E-4`) - derived from `I-N` + `X-N`
>
> **Enforced=No count: 2** - `I-18` (moving-funds safety margin on host wall clock) and `I-19` (deposit-sweep generator deposit-maturity gate on host wall clock). The previous single Enforced=No invariant ("all reservation watcher deadline decisions on wall clock") is now Enforced=Yes (I-11): every reservation deadline decision reads the chain head's block time and skips the decision when the head is unreadable.

---

## 4. Documentation Quality

| Aspect | Status | Notes |
|--------|--------|-------|
| README | Present | `README.adoc` at repo root |
| Godoc (NatSpec analog) | Adequate-Thorough | Self-containment passes committed in the epic; per-file breadcrumbs; fix commits carry G/L finding IDs in comments (`G2-L11`, `G1-S4` style) |
| Spec/Whitepaper | Present (separate branch) | `docs/spec/reservations/**` on `origin/docs/reservations-spec`, not merged into this head; spec-derived claims tagged `(per spec)` |
| Inline Comments | Adequate | Notable gap unchanged: `TODO(test-coverage)` at `pkg/chain/ethereum/tbtc.go:477` - `ValidateReservationAnchorProposal` has no direct unit test |
| Docs changed in fixes | `docs/profiling.md` deleted | pprof removal commit `38f8673e4` deletes the profiling doc alongside the endpoint |

---

## 5. Test Analysis

| Metric | Value | Source |
|--------|-------|--------|
| Test functions | 1,510 `func Test` repo-wide | File scan (first run: 1,228; the fix branch and dev merge added tests, incl. `reservation_chain_time_test.go`, `reservation_regression_test.go`, `action_overdue` tests) |
| Line coverage | Not re-measured on this head (no builds in the frozen worktree) | First run: spv 82.7%, tbtcpg 79.8%, tbtc 77.7%, chain/ethereum 22.1% |
| Branch coverage | Unavailable - Go `cover` reports statement coverage only | - |

### Test Depth

| Category | Count | Coverage |
|----------|-------|----------|
| Unit | 1,510 repo-wide | broad; per-watcher, per-task, and chain-time decision tests (fast/slow host clock fixtures) |
| Benchmarks | ~46 repo-wide | fee math, marshaling, coordination |
| Integration (tagged) | 16 functions in 2 files (`//go:build integration`) | live Electrum / live RPC |
| Simulated multi-signer | 123+ scenario fixtures | coordination windows, reservation anchor/re-anchor, sweeps |
| Stateless Fuzz (`func Fuzz`) | 2 repo-wide | `FuzzDepositSweepProposalRoundTrip` (`pkg/tbtc/fuzz_test.go`), `FuzzEphemeralKeyRoundTrip` (`pkg/internal/pbutils/fuzz_test.go`); the reservation anchor/re-anchor unmarshalers still have no dedicated fuzz target |
| Property-based / stateful harness | 0 | no `rapid`/`quick`/`gopter`; no lifecycle state machine |
| Formal Verification | 0 | none |

### Gaps (ranked by audit impact)

1. **No native fuzz target for the reservation unmarshalers** - the strict protobuf bounds (`pkg/tbtc/marshaling.go:505-645`) and overflow-guarded fee math (`pkg/tbtcpg/fee.go`) take untrusted peer/chain-derived inputs; the two repo-wide fuzz targets cover the sweep proposal and ephemeral-key round trip only.
2. **No stateful/property harness for the coordination -> reservation lifecycle** - lifecycle correctness rests on simulated multi-signer scenarios and chain-time fixtures, not on generated sequences over the wallet/proof state machines (I-4, I-5, I-16).
3. **`pkg/chain/ethereum` statement coverage was 22.1% in the first run and the security-critical `TODO(test-coverage)` at `tbtc.go:477` is unchanged** - `ValidateReservationAnchorProposal` has no direct unit test.
4. **16 integration tests require live Electrum/Ethereum endpoints** and are excluded from default runs, so the Electrum-failure and head-time-unreadable paths are unexercised in CI.

---

## 6. Developer & Git History

> Repo shape: normal_dev - 144 commits in `origin/dev..9f99f364a` touch the 73 in-scope files; 5 unique authors. The fix-epic slice `49c4df13d..9f99f364a` is 214 commits: 16 fix-epic commits + 3 merges by the epic author, and a 195-commit dev-merge tail from `origin/dev`.

### Contributors

| Author | Commits (scope range) | Notes |
|--------|--------:|-------|
| Piotr Roslaniec (+ name variants) | ~140 of 144 in scope range | single-dominant-developer profile unchanged |
| maclane | 4 in range | reviewers/small fixes |
| keep-core-dev, M1 H Watchers Builder | 1 each | large feat commits from the epic |

Single-developer concentration persists: one author's name variants account for the overwhelming majority of scope commits, including all 16 fix-epic commits.

### Review & Process Signals

| Signal | Value | Assessment |
|--------|-------|------------|
| Merge commits on scope files in range | 4 | Includes the two dev-merge merges + fix-branch merges |
| Fix-epic commits since first audit | 16 | Each names its finding IDs in message/comments (G2-L11, G1-S4, ...) |
| Test co-change | Fix commits ship with new test files (`reservation_chain_time_test.go` +298 lines, regression suite +744) | Co-modification discipline |

### File Hotspots (scope range, top)

| File | Modifications | Note |
|------|-------------:|------|
| `pkg/chain/ethereum/tbtc.go` | 29 | Adapter core |
| `pkg/maintainer/spv/reservation_wiring.go` | 24 | Watcher wiring + startup catch-up |
| `pkg/maintainer/spv/reservation_action_timeout_watch.go` | 22 | Timeout watcher (I-11-fixed site) |
| `pkg/tbtcpg/reservation_acceptance.go` | 21 | Acceptance generator |
| `pkg/tbtc/reservation.go` | 21 | Follower anchor/re-anchor actions |
| `pkg/maintainer/spv/reservation_stale_deposit_watch.go` | 21 | Stale-deposit watcher |

### Security-Relevant Fix-Epic Commits

| SHA | Subject | Signal |
|-----|---------|--------|
| `ad0575e0e` | judge reservation deadlines on chain time | I-11 (old) closed; G2-L11/G2-L13 |
| `768707517` | use chain time for tbtcpg reservation deadlines | G1-S4; head read once per pass, pass fails on read failure |
| `146e3206d` | harden reservation watchers and proof loops | G2-L10 abandon, G2-F16/F17/F18/F19, bounded per-tick reads |
| `7944269c6` | alert on reservation actions past their deadline | new Warn + `spv_reservation_action_overdue_total` metric |
| `4ca44edad` | keep reservation lookups bounded and per-key on read errors | X-8 fix: registration lookup failure fails the pass |
| `30dac8c0f` | enforce a protocol fee minimum, re-anchor target age and bounded chain calls | I-9 target-age gate, follower hard-reject floor |
| `fb520f1fe` | only re-anchor to wallets older than the creation period | target-age gate (I-9/E-3) |
| `2a8c1085e` | expose chain head time and wallet parameters to bridge chain users | `GetLatestBlockTimestamp` + `GetWalletParameters` |
| `bc1a0aa10` | reject out-of-range private key shares on unmarshal | beacon DKG hardening |
| `4c9ace3ad` | apply the gas estimate margin with ceiling integer math | `GasEstimateWithMargin` (float sites in `tbtc.go` remain) |
| `bd3e7c53a` | isolate released broadcast sessions from later channels | session-scoped channel lists |
| `38f8673e4` | remove pprof endpoints from the metrics server | attack surface removed |
| `eb8c3d39f` | reject malformed ephemeral public keys at unmarshal | tecdsa DKG/signing hardening |
| `7c8269d6a` | add a deploy check for the paired reservation flags | out-of-band config validation |

### Dangerous Area Evolution

| Security Area | Commits | Key Files |
|--------------|--------:|-----------|
| proof_submission | still hottest | reservation_proof_loop.go, reservation_action_timeout_watch.go, reservation_event_scan.go |
| deadline_timing (NEW) | 6 | tbtc_chain_time.go + all watcher/generator files |
| proposal_validation | 45 | reservation.go, deposit_sweep.go, tbtc.go |
| fee_math | 40+ | fee.go, reservation_acceptance.go, proposal_fee_check.go |
| marshaling | 18+ | marshaling.go, coordination.go, tecdsa/beacon marshaling |

### Forked Dependencies (`go.mod` replaces)

Unchanged from the first run: tss-lib fork, in-tree btcec copy, keep-network/go-electrum fork, threshold-network keep-common v1.7.1-tlabs.0, protobuf dev pin.

### Technical Debt Markers

9 TODO/FIXME/HACK lines in scope (was 21): the security-relevant `TODO(test-coverage)` at `pkg/chain/ethereum/tbtc.go:477` persists; `pkg/tbtcpg/deposit_sweep.go:349` ("Cache immutable data") is new.

### Cross-Reference Synthesis

- **The chain-time re-pointing is the dominant new structural fact**: 6 fix commits (`ad0575e0e`, `768707517`, `146e3206d`, `2a8c1085e`, `fb520f1fe`, `30dac8c0f`) close the old I-11 across all reservation paths; the audit delta file tracks each.
- **`proof_submission` churn now maps to bounded/abandon logic** (`reservationActionAbandoned`, reanchor backoff, wallet-tx cache eviction) rather than wall-clock gates.
- **Marshaling hardening tracks I-15** - the out-of-range key/share rejections landed in the same files the first audit flagged as the fuzz gap; the gap narrowed (2 fuzz targets exist) but the reservation unmarshalers remain uncovered.
- **Activation-block churn still settles on X-6** - `ReservationsActivationBlock` remains the single leader/follower source of truth; the table is still empty at this head.

---

## X-Ray Verdict

**EXPOSED** - the single unenforced wall-clock invariant of the first run is fixed across all reservation paths (deadline decisions now judge on the chain head's block time, skipping on unreadable head), but two non-reservation sites still compare host clock to chain-derived times (`moving_funds.go:479`, `pkg/tbtcpg/deposit_sweep.go:223`), Tests remain FRAGILE (1,510 `func Test` functions but only 2 native fuzz targets - neither covers the reservation unmarshalers - and 0 stateful/property harnesses), and the security-critical `TODO(test-coverage)` at `pkg/chain/ethereum/tbtc.go:477` (`ValidateReservationAnchorProposal` untested) persists; Docs and Access Control reach HARDENED (spec present on `origin/docs/reservations-spec`; roles clear; the client relies on the tECDSA threshold-signing quorum at `reservation.go:746,983` plus the new follower fee hard-reject and target-age gates).

**Structural facts:**
1. 64,468 total nSLOC across the 73 scope files (17,960 in 50 hand-written files in 8 subsystems; 46,508 in 23 generated ABI-binding files out of analysis scope).
2. 1,510 `func Test` functions repo-wide, 2 native `func Fuzz` targets, 0 property-based/stateful harnesses; coverage not re-measured on this head (no builds in the frozen worktree; first run: 82.7% / 79.8% / 77.7% / 22.1%).
3. 67 enforced guards and 31 inferred invariants documented in invariants.md; exactly 2 are Enforced=No (I-18, I-19), both wall-clock vs chain-time sites outside the re-pointed reservation paths.
4. 144 scope-range commits by 5 authors; all 16 fix-epic commits by the epic author; 4 merge commits on scope files in range.
5. The `reservationsActivationBlocks` table is empty at this head, so the feature is disabled by default on every public network; the paired-flag misconfiguration now has an out-of-band deploy check.

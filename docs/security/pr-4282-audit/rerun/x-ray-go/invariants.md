# Invariant Map

> tBTC v2 UTXO Reservations Epic (PR #4282, merged head) | 67 guards | 19 single | 8 cross | 4 economic | 2 not enforced

Line numbers are from the frozen worktree at the merged fix head. The previous run's single Enforced=No invariant ("all reservation watcher deadline decisions on the host wall clock") is now Enforced=Yes (I-11); the two surviving wall-clock sites moved to I-18 and I-19.

---

## 1. Enforced Guards (Reference)

Per-call preconditions. Heading IDs below (`G-N`) are anchor targets from x-ray.md attack surfaces.

#### G-1
`if fee <= 0` · `pkg/tbtc/reservation.go:412` · Enforces strictly positive Bitcoin transaction fee for reservation anchor transactions to prevent zero-fee mempool relay rejections.

#### G-2
`if uint64(fee) > action.TxMaxFee` · `pkg/tbtc/reservation.go:415` · Restricts proposed anchor transaction fee to the maximum authorized fee snapshotted on-chain in the reservation action.

#### G-3
`if anchorValue <= 0` · `pkg/tbtc/reservation.go:419` · Prevents creation of zero or negative value anchor transaction outputs when fee exceeds deposit amount.

#### G-4
`if fee <= 0` · `pkg/tbtc/reservation.go:467` · Enforces strictly positive Bitcoin transaction fee for reservation re-anchor transactions.

#### G-5
`if uint64(fee) > action.TxMaxFee` · `pkg/tbtc/reservation.go:470` · Restricts proposed re-anchor transaction fee to the maximum authorized fee snapshotted on-chain in the re-anchor action.

#### G-6
`if anchorValue <= 0` · `pkg/tbtc/reservation.go:475` · Prevents creation of zero or negative value re-anchor transaction outputs when fee exceeds existing anchor UTXO value.

#### G-7
`if action.ActionType != ReservationActionTypeAcceptance || action.State != ReservationActionStatePending` · `pkg/tbtc/reservation.go:671` · Verifies on-chain action record is an active pending acceptance authorization before follower threshold signing.

#### G-8
`if action.TargetWalletPublicKeyHash != walletPublicKeyHash` · `pkg/tbtc/reservation.go:674` · Prevents signing an acceptance anchor transaction targeting a wallet public key hash other than the local custody wallet.

#### G-9
`if err != nil` · `pkg/tbtc/reservation.go:690` · Requires successful view simulation of `WalletProposalValidator.validateReservationAnchorProposal` before follower approves signing.

#### G-10
`if raa.expiryBlock < reservationActionSigningTimeoutSafetyMarginBlocks` · `pkg/tbtc/reservation.go:742` · Avoids unsigned integer underflow when calculating anchor transaction signing deadline against the 300-block safety margin.

#### G-11
`if action.ActionType != ReservationActionTypeReanchor || action.State != ReservationActionStatePending` · `pkg/tbtc/reservation.go:852` · Verifies on-chain action record is an active pending re-anchor authorization before follower threshold signing.

#### G-12
`if action.TargetWalletPublicKeyHash != rra.proposal.TargetWalletPublicKeyHash` · `pkg/tbtc/reservation.go:855` · Ensures proposed re-anchor target wallet matches the authorized target wallet recorded in the on-chain action snapshot.

#### G-13
`if err != nil` · `pkg/tbtc/reservation.go:927` · Requires successful view simulation of `WalletProposalValidator.validateReservationReanchorProposal` before follower approves signing.

#### G-14
`if rra.expiryBlock < reservationActionSigningTimeoutSafetyMarginBlocks` · `pkg/tbtc/reservation.go:979` · Avoids unsigned integer underflow when calculating re-anchor transaction signing deadline against the 300-block safety margin.

#### G-15
`if !IsReanchorTargetAged(targetWallet.CreatedAt, walletParams.CreationPeriod, headTimestamp, ReanchorTargetAgeTolerance)` · `pkg/tbtc/reservation.go:901` · NEW (fix `a83ba04ef`/`5a7b125fd`): follower refuses to co-sign a re-anchor unless the target wallet has aged out of its creation period, judged on the chain head timestamp; all three reads (`GetWallet`, `GetWalletParameters`, `GetLatestBlockTimestamp`) fail closed into rejection.

#### G-16
`if err := rejectIfProposedWalletTxFeeBelowProtocolMin(raa.proposal.AnchorTxFee, anchorTxSize, "reservation anchor")` · `pkg/tbtc/reservation.go:713` · NEW hard reject: follower refuses to co-sign a reservation anchor transaction whose fee is below the fixed 5 sat/vB protocol minimum for the estimated size.

#### G-17
`if err := rejectIfProposedWalletTxFeeBelowProtocolMin(rra.proposal.ReanchorTxFee, reanchorTxSize, "reservation re-anchor")` · `pkg/tbtc/reservation.go:950` · NEW hard reject for the re-anchor path, same protocol minimum.

#### G-18
`if dsa.proposalExpiryBlock < dsa.signingTimeoutSafetyMarginBlocks` · `pkg/tbtc/deposit_sweep.go:324` · Prevents unsigned integer underflow when computing the deposit sweep signing deadline against the 300-block safety margin.

#### G-19
`if len(proposal.DepositsKeys) != len(proposal.DepositsRevealBlocks)` · `pkg/tbtc/deposit_sweep.go:474` · Rejects sweep proposals whose per-deposit reveal blocks array is misaligned with the deposit key array.

#### G-20
`if confirmations < requiredFundingTxConfirmations` · `pkg/tbtc/deposit_sweep.go:519` · Requires at least 6 Bitcoin funding transaction confirmations per swept deposit before signing.

#### G-21
`if isReserved` · `pkg/tbtc/deposit_sweep.go:620` · Hard-rejects any deposit sweep proposal containing an active reserved deposit UTXO to prevent commingling into the wallet pooled balance; a lookup failure also hard-rejects (fail closed).

#### G-22
`if err := rejectIfProposedWalletTxFeeBelowProtocolMin(proposal.SweepTxFee, check.sweepTxSize, "deposit sweep")` · `pkg/tbtc/deposit_sweep.go:705` · NEW hard reject: below-protocol-minimum sweep fees are refused before signing.

#### G-23
`if rap.AnchorTxFee.Sign() <= 0 || len(rap.AnchorTxFee.Bytes()) > 8` · `pkg/tbtc/marshaling.go:505` · Enforces positive fee bounded by uint64 max size (8 bytes) when marshaling ReservationAnchorProposal.

#### G-24
`if pbMsg.RequestNonce == 0` · `pkg/tbtc/marshaling.go:540` · Rejects unmarshaling ReservationAnchorProposal with an uninitialized or zero request nonce (nonces are strictly 1-indexed).

#### G-25
`if len(pbMsg.DepositFundingTxHash) != 32` · `pkg/tbtc/marshaling.go:543` · Rejects unmarshaling ReservationAnchorProposal with an invalid Bitcoin transaction hash byte length; an all-zero hash is also rejected.

#### G-26
`if rrp.ReanchorTxFee.Sign() <= 0 || len(rrp.ReanchorTxFee.Bytes()) > 8` · `pkg/tbtc/marshaling.go:578` · Enforces positive fee bounded by uint64 max size when marshaling ReservationReanchorProposal.

#### G-27
`if len(pbMsg.ReservationKey) > 32` · `pkg/tbtc/marshaling.go:607` · Rejects unmarshaling ReservationReanchorProposal with an oversized reservation key byte slice exceeding uint256 max size.

#### G-28
`if pbMsg.RequestNonce == 0` · `pkg/tbtc/marshaling.go:613` · Rejects unmarshaling ReservationReanchorProposal with a zero request nonce.

#### G-29
`if len(pbMsg.TargetWalletPublicKeyHash) != 20` · `pkg/tbtc/marshaling.go:625` · Rejects unmarshaling ReservationReanchorProposal with an invalid target wallet public key hash length; an all-zero PKH is also rejected.

#### G-30
`if key.Sign() <= 0` · `pkg/tbtc/marshaling.go:637` · Rejects a re-anchor reservation key whose big.Int value is non-positive.

#### G-31
`if rrp.ReanchorTxFee.Sign() <= 0` · `pkg/tbtc/marshaling.go:645` · Rejects a re-anchor fee that is zero or negative after byte decoding.

#### G-32
`if lockAcquired := ce.lock.TryAcquire(1); !lockAcquired` · `pkg/tbtc/coordination.go:415` · Prevents concurrent coordination procedure runs on the same wallet executor instance.

#### G-33
`if leaderID != message.senderID` · `pkg/tbtc/coordination.go:898` · Detects and attributes FaultLeaderImpersonation when a coordination message originates from a non-leader sender.

#### G-34
`if !slices.Contains(actionsAllowed, message.proposal.ActionType())` · `pkg/tbtc/coordination.go:913` · Detects and attributes FaultLeaderMistake when leader proposes an action not allowed in the current window checklist.

#### G-35
`if _, ok := wd.actions[key]; ok` · `pkg/tbtc/wallet.go:218` · Prevents concurrent action execution on the same wallet key in walletDispatcher, returning errWalletBusy.

#### G-36
`if bufferedFloorRate > math.MaxInt64/txVsize` · `pkg/tbtcpg/fee.go:169` · Checked arithmetic guard preventing int64 overflow when computing the buffered floor product.

#### G-37
`if bufferedFloorProduct > feeLimit` · `pkg/tbtcpg/fee.go:182` · Returns ErrMaxFeeTooLow when the minimum safe fee floor exceeds the maximum total fee allowed on-chain.

#### G-38
`if rate > maxRateForBuffer` · `pkg/tbtcpg/fee.go:147` · Checked arithmetic guard preventing int64 overflow during fee buffer multiplier ceiling calculations.

#### G-39
`if totalFee < 0 || uint64(totalFee) > maxTotalFee` · `pkg/tbtcpg/fee.go:245` · `estimateCappedFee` rejects a computed fee that underflows or exceeds the on-chain maximum total fee.

#### G-40
`if uint64(totalFee) > txMaxFee` · `pkg/tbtcpg/fee.go:288` · Rejects a reservation fixed-size fee estimate when the raw fee exceeds the snapshotted action TxMaxFee.

#### G-41
`if wallet.State != tbtc.StateLive && wallet.State != tbtc.StateMovingFunds` · `pkg/tbtcpg/reservation_acceptance.go:366` · Restricts reservation acceptance proposal generation strictly to wallets in Live or MovingFunds state.

#### G-42
`if uint64(now.Unix())+uint64(reservationRequestTimeoutSafetyMarginSeconds) >= uint64(requestEvent.TimeoutAt)` · `pkg/tbtcpg/reservation_acceptance.go:459` · NEW chain-time form (fix `7c34c4e12`): skips generating an acceptance proposal for a request event within 2 hours of its timeout, judged on the chain head timestamp read at pass start (`reservation_acceptance.go:433`).

#### G-43
`if action.ActionType != tbtc.ReservationActionTypeAcceptance || action.State != tbtc.ReservationActionStatePending` · `pkg/tbtcpg/reservation_acceptance.go:505` · Skips candidate deposits whose on-chain action generation drifted away from pending acceptance.

#### G-44
`if action.TargetWalletPublicKeyHash != walletPublicKeyHash` · `pkg/tbtcpg/reservation_acceptance.go:518` · Skips candidate deposits whose pending acceptance targets another wallet.

#### G-45
`if uint64(now.Unix())+uint64(reservationRequestTimeoutSafetyMarginSeconds) >= uint64(action.TimeoutAt)` · `pkg/tbtcpg/reservation_acceptance.go:535` · NEW chain-time form: skips a pending generation within 2 hours of its action-level timeout.

#### G-46
`if !now.After(matureAt)` · `pkg/tbtcpg/reservation_acceptance.go:617` · Ensures candidate deposits have reached minimum age since reveal, measured on the chain head timestamp (was host wall clock before fix `7c34c4e12`).

#### G-47
`if lookup.confirmations < tbtc.DepositSweepRequiredFundingTxConfirmations` · `pkg/tbtcpg/reservation_acceptance.go:896` · Requires at least 6 Bitcoin funding transaction confirmations before assembling an acceptance proposal.

#### G-48
`if minPlusFee < action.MinAmount || depositRequest.Amount < minPlusFee` · `pkg/tbtcpg/reservation_acceptance.go:945` · Verifies the deposit amount covers the snapshotted action MinAmount plus the estimated anchor transaction fee, with overflow protection.

#### G-49
`if walletChainData.State != tbtc.StateMovingFunds && walletChainData.State != tbtc.StateClosing` · `pkg/tbtcpg/reservation_reanchor.go:170` · Restricts reservation re-anchor proposal generation to retiring wallets in MovingFunds or Closing state.

#### G-50
`if uint64(headTime.Unix())+uint64(reservationRequestTimeoutSafetyMarginSeconds) >= uint64(action.TimeoutAt)` · `pkg/tbtcpg/reservation_reanchor.go:306` · NEW chain-time form: skips resuming a pending re-anchor generation within 2 hours of its timeout, judged on the chain head timestamp read once per pass (`reservation_reanchor.go:227`).

#### G-51
`if uint64(headTime.Unix()) < uint64(reservation.ReanchorCooldownUntil)` · `pkg/tbtcpg/reservation_reanchor.go:438` · NEW chain-time form: enforces the re-anchor cooldown period timestamp against the chain head instead of the host clock.

#### G-52
`if anchorValue <= params.ReservationTxMaxFee+params.ReservationMinAmount` · `pkg/tbtcpg/reservation_reanchor.go:447` · Skips re-anchoring a reservation whose anchor UTXO value cannot cover the transaction max fee plus the minimum reservation amount.

#### G-53
`if errors.As(err, &postSubmitErr)` · `pkg/tbtcpg/reservation_reanchor.go:504` · Terminates the re-anchor task pass immediately after an on-chain request transaction submission, enforcing at most one request per pass.

#### G-54
`if reservation.State != tbtc.ReservationStateActive` · `pkg/tbtcpg/reservation_reanchor.go:698` · Requires the reservation to be Active before submitting a new on-chain requestReservationReanchor transaction.

#### G-55
`if !tbtc.IsReanchorTargetAged(s.createdAt[candidate], s.creationPeriod, s.headTime, 0)` · `pkg/tbtcpg/reservation_reanchor.go:1101` · NEW (fix `5a7b125fd`): excludes a candidate re-anchor target wallet that has not aged out of its creation period, measured on the chain head; the target search sets `waitingOnAgedTarget` instead of proposing.

#### G-56
`if count+1 > s.maxReservationsPerWallet` · `pkg/tbtcpg/reservation_reanchor.go:1128` · Excludes candidate target wallets that would exceed the per-wallet reservation count cap.

#### G-57
`if amount+anchorValue > s.maxReservationsAmountPerWallet` · `pkg/tbtcpg/reservation_reanchor.go:1148` · Excludes candidate target wallets that would exceed the aggregate reservation amount cap (zero cap disables the check, matching on-chain behavior).

#### G-58
`if wallet.State != tbtc.StateLive` · `pkg/tbtcpg/reservation_reanchor.go:1265` · Requires candidate re-anchor target wallets to be in StateLive.

#### G-59
`if !headTime.IsZero() && uint64(max(headTime.Unix(), 0)) > uint64(action.TimeoutAt)+uint64(action.TermSeconds)` · `pkg/maintainer/spv/reservation_proof_loop.go:494` · NEW chain-time form: evicts a TimedOut acceptance generation from the SPV proof scan only once the chain head has passed the snapshotted custody term; when the head is unreadable the generation is kept, not evicted.

#### G-60
`if reservationActionAbandoned(action, headTime)` · `pkg/maintainer/spv/reservation_proof_loop.go:528` · NEW (fix `41fb820a5`, G2-F16): evicts a Pending acceptance generation whose notification evidently will not land, once the chain head is past `TimeoutAt + 4*(TimeoutAt-RequestedAt)`; zero head time disables the eviction.

#### G-61
`if _, submitted := submittedReservations[reservationKey]; submitted` · `pkg/maintainer/spv/reservation_proof_loop.go:1023` · Enforces at most one re-anchor proof submission per reservation key per proof loop pass.

#### G-62
`if item.notifiedAt != 0 && reservationActionAbandoned(action, headTime)` · `pkg/maintainer/spv/reservation_action_timeout_watch.go:398` · NEW (G2-L10): stops re-reading a tracked action that was notified but stays Pending past its abandonment bound in chain time, bounding per-tick RPC volume.

#### G-63
`if headTime.IsZero() { continue } ... if headNow >= deadline` · `pkg/maintainer/spv/reservation_action_timeout_watch.go:461,465` · NEW chain-time form (G2-L11): the timeout-notify decision is judged on the chain head's block time; when the head is unreadable no notification is attempted; the node clock (`now`) is used only for the 10-minute renotify backoff (line 408, with the `now < item.notifiedAt` underflow guard) and the `notifiedAt` stamp (line 481). The overdue-action alert (`alertOverdueAction`, line 466) fires on the same chain-time decision.

#### G-64
`if headTime.IsZero() { return StaleDepositResolutionKeep, nil } ... if headNow <= deadline` · `pkg/maintainer/spv/reservation_stale_deposit_watch.go:273,281` · NEW chain-time form (G2-L13): the stale-deposit refund-deadline gate is judged on the chain head; when the head is unreadable the deposit is kept tracked rather than notified or evicted on a guess.

#### G-65
`if pkh != action.TargetWalletPublicKeyHash` · `pkg/maintainer/spv/reservation_reanchor_proof.go:255` · Re-verifies the Bitcoin transaction output public key hash matches the authorized target wallet snapshot immediately before proof submission.

#### G-66
`switch action.State { case tbtc.ReservationActionStatePending, tbtc.ReservationActionStateTimedOut: default: ... }` · `pkg/maintainer/spv/reservation_reanchor_proof.go:272` · Restricts SPV proof submission strictly to Pending or TimedOut action states, rejecting settled or superseded actions; zero required confirmations (line 207), nil reservation key (line 213) and zero request nonce (line 219) are also rejected.

#### G-67
`if reservation.State != tbtc.ReservationStateActive` ... `if requireDissolutionEligible && (reservation.DissolutionEligibleAt == 0 || uint64(headTime.Unix()) < uint64(reservation.DissolutionEligibleAt))` · `pkg/maintainer/spv/reservation_stranding_watch.go:246,256` · NEW (G2-F17): stranding notifications are restricted to Active reservations; for Closing wallets the notification additionally waits until the chain head time reaches the reservation's `DissolutionEligibleAt`, deferring to a later pass otherwise.

---

## 2. Inferred Invariants (Single-Component)

Inferred invariants are derived from structural analysis of the source code. Each block cites one of five extraction methods in its `Derivation` field:
- **delta-pair analysis** - two or more variables in the same function body that change by equal-and-opposite amounts.
- **Guard lift** - a precondition on persistent state promoted to a global property by verifying all write sites.
- **State-machine edge** - discrete transitions with guards preventing reversal.
- **Temporal predicate** - constraints tied to timestamps, block heights, or deadlines.
- **Comment-stated global property** - developer-asserted invariants confirmed or contradicted by the code scan.

Each block is classified into one of five categories: `Conservation` / `Bound` / `Ratio` / `StateMachine` / `Temporal`.

---

#### I-1

`StateMachine` · Enforced: **Yes**

> Every reservation anchor and re-anchor Bitcoin transaction is a strict 1-input-1-output transaction spending either the revealed deposit UTXO or the previous anchor UTXO into a new anchor output (per spec).

**Derivation** - guard-lift: `len(transaction.Inputs) == 1 && len(transaction.Outputs) == 1` + write-sites: `AssembleReservationAnchorTransaction` (`pkg/tbtc/reservation.go:412-419`), `AssembleReservationReanchorTransaction` (`pkg/tbtc/reservation.go:467-475`), `spentOutputAsUtxo` (`pkg/maintainer/spv/reservation_reanchor_proof.go:82`), `parseReservationTransaction` (`pkg/maintainer/spv/reservation_reanchor_proof.go:130`), `isMatchingReservationAcceptanceTransaction` / `isMatchingReservationReanchorTransaction` (`pkg/maintainer/spv/reservation_proof_loop.go:646,1180`).

**If violated** - Multi-input or multi-output transactions would commingle UTXO lineages, violate Bridge SPV proof decoding assumptions, and allow unauthorized diversion of reserved collateral.

---

#### I-2

`Conservation` · Enforced: **Yes**

> For any reservation anchor or re-anchor transaction, `anchorValue = inputUtxo.Value - fee` with `anchorValue > 0`. Minted tBTC equals the deposit amount at acceptance and remains invariant across re-anchors while in-kind fee debt tracks Bitcoin network fees deducted from the anchor (per spec).

**Derivation** - delta-pair: `pkg/tbtc/reservation.go:412-419` (`anchorValue := deposit.Utxo.Value - fee`) and `pkg/tbtc/reservation.go:467-475` (`anchorValue := anchorUtxo.Value - fee`).

**If violated** - Anchor values would diverge from minted debt or underflow to zero, leading to uncollateralized tBTC or unspendable Bitcoin dust outputs.

---

#### I-3

`Bound` · Enforced: **Yes**

> Active reserved deposit UTXOs (`IsReservedDeposit == true`) are strictly forbidden from being included in deposit sweep transactions, preventing commingling of reserved funds with the wallet main UTXO pool (per spec).

**Derivation** - guard-lift: `pkg/tbtc/deposit_sweep.go:620` (`if isReserved { return error }`; a lookup failure also fails closed at line 613) + write-sites: `assembleDepositSweepTransaction` (`pkg/tbtc/deposit_sweep.go:818-875`), `pkg/tbtcpg/deposit_sweep.go:268-275` (generator pre-filter, gated on `reservationsActive`).

**If violated** - Reserved deposits would be swept into pooled wallet balances, irreversibly destroying the 1-in-1-out reservation lineage and leaving the reservation unanchorable or double-credited.

---

#### I-4

`StateMachine` · Enforced: **Yes**

> A wallet executes at most one action (`depositSweep`, `redemption`, `movingFunds`, `reservationAnchor`, `reservationReanchor`) at any time; concurrent dispatches for the same wallet key fail with `errWalletBusy`.

**Derivation** - StateMachine: `pkg/tbtc/wallet.go:218-228` (`wd.actions[key] = actionType`) and `pkg/tbtc/wallet.go:247` (`delete(wd.actions, key)` in defer). All dispatch paths enter via `walletDispatcher.dispatch`.

**If violated** - Concurrent signing actions on the same wallet would cause Bitcoin UTXO race conditions, double spends of the wallet main UTXO, or inconsistent state transitions.

---

#### I-5

`StateMachine` · Enforced: **Yes**

> Every reservation action generation is uniquely identified by `(reservationKey, requestNonce)`. Settleable state is limited to `Pending` (or `TimedOut` during the term window); terminal states are irreversibly evicted and cannot be proven (per spec).

**Derivation** - StateMachine edge: `pkg/maintainer/spv/reservation_proof_loop.go:494-528` (late-window and abandon eviction in chain time) and `pkg/maintainer/spv/reservation_reanchor_proof.go:272-279` (settleable-state switch at submit time).

**If violated** - Stale or superseded action generations could be proven after timeout or replacement, minting duplicate tBTC or re-anchoring to obsolete target wallets.

---

#### I-6

`Temporal` · Enforced: **Yes**

> Proposal generators refuse to propose or resume acceptance or re-anchor actions within 2 hours (`7200s`) of `action.TimeoutAt`, and the margin is judged on the chain head's block time, not the host wall clock (per spec; fix `7c34c4e12`).

**Derivation** - temporal: `pkg/tbtcpg/reservation_acceptance.go:433,459,535` (`now` from `chain.GetLatestBlockTimestamp()`) and `pkg/tbtcpg/reservation_reanchor.go:227,306` (`headTime` read once per pass; the pass fails when the read fails).

**If violated** - Proposals broadcast near the timeout boundary would be mined after on-chain timeout expiration, allowing watchers to trigger timeout notices while the Bitcoin transaction is in flight; a fast host clock could mature or time out generations the chain would not yet accept.

---

#### I-7

`Bound` · Enforced: **Yes**

> Every proposed wallet transaction fee rate is floored at `MinWalletTxSatPerVByteFee` (operator-tunable, clamped at startup to the fixed `ProtocolMinWalletTxSatPerVByteFee = 5`) with a `WalletTxFeeBufferPercent` (default 25%) buffer, bounded above by on-chain `TxMaxFee`, and every follower path hard-rejects fees below the protocol minimum.

**Derivation** - guard-lift: `applyWalletTxFeeFloor` (`pkg/tbtcpg/fee.go:136,147,169,182`) + write-sites: `estimateReservationFixedSizeTxFee` (`pkg/tbtcpg/fee.go:288`), `estimateCappedFee` (`pkg/tbtcpg/fee.go:245`), startup clamp `pkg/tbtc/tbtc.go:211` + follower hard reject `pkg/tbtc/proposal_fee_check.go:183-204` (called from `reservation.go:713,950`, `deposit_sweep.go:705`, `moving_funds.go:377`, `redemption.go:399`).

**If violated** - Transactions broadcast with sub-floor fee rates would get stuck in Bitcoin mempools, and fees exceeding `TxMaxFee` would cause on-chain SPV proof and proposal validation reverts.

---

#### I-8

`StateMachine` · Enforced: **Yes**

> A coordination pass on a `MovingFunds` or `Closing` wallet submits at most one on-chain `RequestReservationReanchor` transaction, terminating the pass immediately upon submission.

**Derivation** - StateMachine edge: `pkg/tbtcpg/reservation_reanchor.go:504-506` (error catch on `*errReservationReanchorPostSubmitFailure`) + write-site: `rrt.chain.RequestReservationReanchor` (`pkg/tbtcpg/reservation_reanchor.go:724`).

**If violated** - A single coordination pass could flood the host chain with redundant re-anchor requests for multiple reservations, wasting gas and exceeding wallet reservation caps.

---

#### I-9

`Bound` · Enforced: **Yes**

> Re-anchor proposal generation strictly filters candidate target wallets to those in `StateLive` whose current reservation count does not exceed `maxReservationsPerWallet`, whose aggregate amount does not exceed `maxReservationsAmountPerWallet`, and which have aged out of their wallet creation period (per spec; target-age gate added in fixes `5a7b125fd`/`a83ba04ef`).

**Derivation** - guard-lift: `pkg/tbtcpg/reservation_reanchor.go:1128` (`count+1 > maxCount`), `1148` (`amount+anchorValue > maxAmount`), `1265` (`wallet.State != StateLive`), `1101` (`!tbtc.IsReanchorTargetAged(...)`); target search `reanchorTargetSearch.pick` (`pkg/tbtcpg/reservation_reanchor.go:1091-1127`), and the follower co-sign mirror `pkg/tbtc/reservation.go:901-916`.

**If violated** - Re-anchor requests targeting overloaded, non-live, or brand-new wallets would concentrate custody in wallets that may themselves be retiring or immature, or revert on-chain at the caps.

---

#### I-10

`StateMachine` · Enforced: **Yes**

> The stale deposit watcher never resolves a deposit as `Notified` or `Drop` based on local notification transaction submission alone; tracking is retired only when `ReservedDepositWallet` is observed cleared to `0x00...00` on-chain.

**Derivation** - StateMachine latch: `pkg/maintainer/spv/reservation_stale_deposit_watch.go:318-330` (zero wallet field from chain read is the completion signal; `notifiedAt` presence selects Notified vs Drop). All retirement paths require the chain read.

**If violated** - Dropped or reorganized notification transactions would cause the maintainer to forget unreleased stale deposits, permanently locking user refund paths.

---

#### I-11

`Temporal` · Enforced: **Yes** (was the single Enforced=No invariant of the first run; closed by fix `a17044076`/`7c34c4e12`/`41fb820a5`)

> All reservation watcher, proof-loop, and generator deadline decisions compare the chain head's block time (via `GetLatestBlockTimestamp`) against contract timestamps (`action.TimeoutAt`, `refundDeadline`, `ReanchorCooldownUntil`, `DissolutionEligibleAt`, deposit maturity). The node wall clock remains in use only for the 10-minute renotify backoff and `notifiedAt` stamps. When the chain head time cannot be read, deadline decisions are skipped for that pass - nothing is notified and nothing is evicted on a guess.

**Derivation** - temporal, all chain-time sites: `pkg/maintainer/spv/reservation_action_timeout_watch.go:356,461,465` (timeout-notify gate), `pkg/maintainer/spv/reservation_stale_deposit_watch.go:272-281,435` (refund gate + first-notify stagger), `pkg/maintainer/spv/reservation_proof_loop.go:466,494,939,1143-1172` (late-window eviction + abandon bound via `reservationChainHeadTime`/`reservationActionAbandoned`), `pkg/maintainer/spv/reservation_stranding_watch.go:177,256` (dissolution-eligible gate), `pkg/tbtcpg/reservation_acceptance.go:433`, `pkg/tbtcpg/reservation_reanchor.go:227`, `pkg/tbtc/reservation.go:893` (target age). The host-clock sites that survive are `pkg/maintainer/spv/reservation_action_timeout_watch.go:224` (nowFn, backoff/stamp only), `reservation_stale_deposit_watch.go:723` and `reservation_wiring.go:461` (tick token only).

**If violated** - With a skewed host clock, watchers could no longer fire `NotifyReservationAcceptanceTimedOut` / `NotifyStaleReservedDeposit` before the on-chain `block.timestamp` threshold (reverted, wasted gas) or defer them past it; this invariant now holds for every reservation path. The two remaining host-clock sites outside the reservation lifecycle are tracked separately (I-18, I-19).

---

#### I-12

`StateMachine` · Enforced: **Yes**

> Stranding notifications are submitted exclusively for reservations in `Active` state belonging to `Closed` or `Terminated` wallets; reservations with pending action generations are deferred to the action-timeout watcher, and Closing-wallet notifications additionally wait for the reservation's `DissolutionEligibleAt` in chain time.

**Derivation** - StateMachine edge: `pkg/maintainer/spv/reservation_stranding_watch.go:237` (ActionPending defer) and `:246` (Active-only) + write-sites: `checkReservationStrandingForWallet` / `checkStranding` (`pkg/maintainer/spv/reservation_stranding_watch.go:97-270`), dissolution gate `:256-258`.

**If violated** - Premature stranding notifications would preempt in-flight re-anchor or acceptance settlements, forcing unnecessary manual reconciliation costs on reservation owners.

---

#### I-13

`Bound` · Enforced: **Yes**

> A `MovingFunds` wallet is notified as below-dust to the Bridge only after all custodied reservations have been re-anchored away and the wallet's main UTXO value is below the on-chain dust threshold.

**Derivation** - guard-lift: `pkg/tbtcpg/reservation_reanchor.go:194-199` (notify only when the reservation list is empty), `1340-1361` (`MainUtxoHash` zero or `walletMainUtxo.Value < int64(params.DustThreshold)`) + write-sites: `rrt.chain.NotifyMovingFundsBelowDust` (`pkg/tbtcpg/reservation_reanchor.go:1455`).

**If violated** - Winding-down wallets holding active reservation anchors would close prematurely, or unreserved wallets would trigger redundant dust notifications.

---

#### I-14

`Bound` · Enforced: **Yes**

> Signers reject signing proposals whose `expiryBlock` is within 300 blocks of the start block or underflows `reservationActionSigningTimeoutSafetyMarginBlocks = 300`.

**Derivation** - temporal / bound: `pkg/tbtc/reservation.go:742,979` (`expiryBlock < 300`) and `pkg/tbtc/deposit_sweep.go:324`. All signing invocation sites gate on this predicate.

**If violated** - Integer underflow in signing deadline math would cause signers to set infinite signing deadlines or accept expired coordination proposals.

---

#### I-15

`Bound` · Enforced: **Yes**

> Peer-supplied wire proposal protobufs with non-positive fees, oversized fee integers (>8 bytes), oversized keys (>32 bytes), zero nonces, all-zero hashes, or mismatched public key hash lengths are rejected during unmarshaling; ephemeral public key maps and beacon private key shares are validated as in-range curve values at unmarshal time.

**Derivation** - guard-lift: `pkg/tbtc/marshaling.go:505,540,543,547,557,578,607,613,616,619,625,632,637,645` + write-sites: `unmarshalCoordinationProposal` (`pkg/tbtc/marshaling.go:219-260`); out-of-range key rejections: `pkg/tecdsa/dkg/marshaling.go` and `pkg/tecdsa/signing/marshaling.go` (compressed-point parse, fix `c5688f92c`), `pkg/beacon/dkg/marshaling.go:76` (`privateKeyShare.Sign() <= 0 || privateKeyShare.Cmp(bn256.Order) >= 0`, fix `f25747bc7`).

**If violated** - Malformed or malicious peer messages could crash nodes via nil pointer dereferences or trigger out-of-bounds memory allocations during proposal processing.

---

#### I-16

`StateMachine` · Enforced: **Yes**

> The SPV maintainer proof loop submits at most one acceptance or re-anchor proof per reservation key per execution pass, preventing duplicate transaction collisions on-chain.

**Derivation** - StateMachine latch: `pkg/maintainer/spv/reservation_proof_loop.go:1023` (re-anchor pass `submittedReservations[reservationKey]`) with the acceptance-pass equivalent in the same loop; re-anchor backoff (`reanchorGenerationBackedOff`, line 828-838) additionally caps repeat attempts at 32 passes.

**If violated** - Multiple concurrent proof submissions for the same reservation would collide in the mempool, causing all but the first to revert on-chain and waste operator funds.

---

#### I-17

`Bound` · Enforced: **Yes**

> All contract calls and event subscriptions to `ReservationRouter` must use the deployed `Bridge` contract address; standalone router address calls operate on uninitialized storage and fail (per spec).

**Derivation** - comment-stated global property: `pkg/chain/ethereum/tbtc.go:340-345` ("The router code only ever executes via Bridge.fallback's delegatecall, so the binding MUST be constructed against the Bridge address"); the binding is constructed at the Bridge address and every reservation view/write routes through it (`pkg/chain/ethereum/tbtc.go:407,434,466,840+`).

**If violated** - Direct calls to the deployed ReservationRouter address would read empty storage mappings (returning zero balances and uninitialized parameters) and write to orphaned storage.

---

#### I-18

`Temporal` · Enforced: **No**

> The moving-funds safety-margin gate compares the host wall clock against the wallet's on-chain `MovingFundsRequestedAt` plus the computed margin (24 h, or 14 d when the wallet is a pending moving-funds target), clamped to half the on-chain moving-funds timeout.

**Derivation** - temporal: `pkg/tbtc/moving_funds.go:477-481` (`safetyMarginExpiresAt := walletChainData.MovingFundsRequestedAt.Add(safetyMargin)` followed by `if time.Now().Before(safetyMarginExpiresAt) { return fmt.Errorf("safety margin in force") }`). This gate was not among the deadlines re-pointed to chain time by the audit fixes (the fix set covered the reservation lifecycle only); it is a pre-existing decision site.

**If violated** - A host clock running fast ahead of chain time would expire the margin early and allow a moving-funds transaction to be proposed while last-minute deposits or other wallets' moved funds are still landing; a slow host clock defers the move. The on-chain `ValidateMovingFundsProposal` is the backstop for the wrong-direction case.

---

#### I-19

`Temporal` · Enforced: **No**

> The deposit-sweep proposal generator decides deposit maturity (minimum deposit age since reveal) on the host wall clock, not on chain time: `timeNow := time.Now()` at pass start is used for the `!timeNow.After(matureAt)` skip.

**Derivation** - temporal: `pkg/tbtcpg/deposit_sweep.go:223` (`timeNow := time.Now()`) and `:288` (`if !timeNow.After(matureAt)` where `matureAt := depositRequest.RevealedAt.Add(depositMinAge)`). The reservation tasks in the same checklist window use `GetLatestBlockTimestamp` (fix `7c34c4e12`); the sweep generator was not re-pointed.

**If violated** - With a fast host clock the generator proposes a sweep for a deposit the on-chain `ValidateDepositSweepProposal` still considers too young, wasting the proposal round; with a slow host clock it misses mature deposits. The asymmetry between the two generators' clock domains in the same window is the residual gap.

---

## 3. Inferred Invariants (Cross-Component)

Trust assumptions that span subsystem boundaries within the codebase. Each block cites producer-side and consumer-side implementations.

---

#### X-1

Enforced: **Yes**

> The deposit sweep proposal generator and follower validator agree that active reserved deposits (`IsReservedDeposit == true`) are excluded from sweep transactions, preventing reserved funds from being commingled into the wallet main UTXO pool (per spec).

**Producer side** (Generator) - `pkg/tbtcpg/deposit_sweep.go:268-275` - Filters out deposits targeting the reservation vault that return `isReserved == true` from `chain.IsReservedDeposit`, gated on `reservationsActive`.

**Consumer side** (Validator) - `pkg/tbtc/deposit_sweep.go:594-628` - Independently queries `chain.IsReservedDeposit` and hard-rejects any proposal containing an active reserved deposit before signing; a lookup failure also fails closed.

**If violated** - Reserved deposits would be swept into pooled wallet balances, irreversibly destroying the 1-in-1-out reservation lineage and leaving the reservation unanchorable or double-credited.

---

#### X-2

Enforced: **Yes**

> Follower node signers and proposal generators agree that proposals must simulate successfully against `WalletProposalValidator` on-chain before signing or broadcasting.

**Producer side** (Generator) - `pkg/tbtcpg/reservation_acceptance.go:1192` / `pkg/tbtcpg/reservation_reanchor.go:825` - Runs `chain.ValidateReservationAnchorProposal` / `chain.ValidateReservationReanchorProposal` before emitting proposals.

**Consumer side** (Validator) - `pkg/tbtc/reservation.go:677-692` / `pkg/tbtc/reservation.go:920-924` - Re-runs the validator view calls before signing, and adds the target-age gate (`:901-916`) and protocol fee hard reject (`:713,950`) for re-anchors.

**If violated** - Proposals containing invalid fee calculations, unconfirmed funding transactions, mismatched wallet assignments, or immature target wallets would be signed by honest followers, risking fraud challenge slashing.

---

#### X-3

Enforced: **Yes**

> The SPV maintainer proof submitter re-verifies the on-chain target wallet public key hash authorization immediately before proof submission, preventing stale or hijacked destination proofs.

**Producer side** (Proof Loop) - `pkg/maintainer/spv/reservation_proof_loop.go` candidate matching (PKH check at the matching helpers) - Matches candidate Bitcoin transaction output PKH against the event log snapshot.

**Consumer side** (Submitter) - `pkg/maintainer/spv/reservation_reanchor_proof.go:255-279` - Re-reads the on-chain action record `GetReservationAction` and asserts `pkh == action.TargetWalletPublicKeyHash` and the settleable state immediately before tx submission.

**If violated** - Proofs for transactions paying wrong destination addresses could be submitted if chain state transitioned between discovery and submission, reverting on-chain and wasting gas.

---

#### X-4

Enforced: **Yes** (upgraded from warn-only; mixed-version divergence remains)

> The follower-side fee check is a two-layer policy: a hard reject at the fixed protocol minimum (5 sat/vB x (vsize-1)) that no follower crosses, plus a log-only warn at the operator's own bare floor. The leader hard-bounds its fee with `applyWalletTxFeeFloor`; the on-chain `WalletProposalValidator` bounds fees from above.

**Producer side** (Generator) - `pkg/tbtcpg/fee.go:136,147,169,182` - Applies the buffered fee floor during proposal generation (commit `a83ba04ef` added the overflow-checked ceiling math and the fixed protocol minimum).

**Consumer side** (Follower) - `pkg/tbtc/proposal_fee_check.go:183-258` - `rejectIfProposedWalletTxFeeBelowProtocolMin` hard-rejects below the protocol minimum on every wallet-tx path; `warnIfProposedWalletTxFeeBelowBareFloor` logs below the operator floor.

**If violated** - Unpatched nodes in a mixed-version rollout would still co-sign below-protocol-minimum fees (the hard reject exists only in this head), so the protocol floor holds only once every follower upgrades; the one-vByte allowance keeps honest leaders from tripping followers that estimate size one vByte larger.

---

#### X-5

Enforced: **Yes**

> Proposal generators and on-chain proposal validators agree on a strict 2-hour timeout safety margin (`7200s`), refusing to propose or sign actions near their timeout expiration, and both judge the margin on chain time (per spec).

**Producer side** (Generator) - `pkg/tbtcpg/reservation_acceptance.go:459,535` and `pkg/tbtcpg/reservation_reanchor.go:306` - Drop candidate actions where `chainHeadTime + 7200 >= action.TimeoutAt`.

**Consumer side** (On-Chain Validator) - `pkg/chain/ethereum/tbtc.go:491,582` call `WalletProposalValidator.validateReservationAnchorProposal` / `validateReservationReanchorProposal` (tbtc-v2, out of scope; line numbers not verifiable from this repo) - Enforce the 7200s `REQUEST_TIMEOUT_SAFETY_MARGIN` against `block.timestamp` during view validation (per spec).

**If violated** - Proposals accepted by generators would fail on-chain validation, or transactions signed near the boundary would time out before mining on Bitcoin; with the re-pointing to chain time, a fast host clock can no longer produce such proposals.

---

#### X-6

Enforced: **Yes**

> Coordination leader and followers deterministically derive identical action checklists from network block height, avoiding spurious fault attributions.

**Producer side** (Leader) - `pkg/tbtc/coordination.go:714` - Adds `ActionReservationAnchor` and `ActionReservationReanchor` to the checklist when `coordinationBlock >= ReservationsActivationBlock(ethereumNetwork)`; the same predicate is passed as `ReservationsActive` into the proposal request (line 747).

**Consumer side** (Follower) - `pkg/tbtc/coordination.go:913` - Validates the proposed action type against its own derived allowed actions, attributing `FaultLeaderMistake` if the leader proposes an unlisted action.

**If violated** - Followers with mismatched activation configuration would falsely accuse honest coordination leaders of protocol mistakes, corrupting operator performance metrics.

---

#### X-7

Enforced: **Yes** (enforced on-chain, out of scope)

> Stranding watchers assume idempotency on the host chain contract (`ReservationRouter.notifyReservationStranded`), allowing multiple concurrent watcher paths without corruption.

**Producer side** (Watchers) - `pkg/maintainer/spv/reservation_wiring.go` (startup catch-up, second-chance retry, `OnWalletClosed` subscription) and `pkg/maintainer/spv/reservation_action_timeout_watch.go:755` (stranding recheck queue drained before notify) - four independent concurrent triggers invoke stranding checks without global locking; a failed notify or recheck re-queues the wallet (lines 841-872, G2-F18/F19).

**Consumer side** (Bridge Contract) - `ReservationRouter.sol:notifyReservationStranded` - treats notifications on already-stranded reservations as a no-op without reverting.

**If violated** - Concurrent watcher notifications on wallet closure would revert transactions and trigger false error alerts.

---

#### X-8

Enforced: **Yes** (failure mode upgraded: bounded and per-key, no genesis fallback)

> Wallet registration block memoization bounds historical event query windows to wallet lifetime, preventing genesis-to-tip RPC scans; a failed registration lookup now fails the pass instead of degrading to a full-history scan, and one unreadable reservation no longer aborts the lookup for the rest of the wallet (fix `a29a69690`).

**Producer side** (Memoizer) - `pkg/chain/ethereum/tbtc.go:1390-1488` - `earliestWalletRegistrationBlock` caches in `walletRegistrationBlockCache` and walks back in doubling windows; errors propagate instead of falling back to `startBlock = 0`.

**Consumer side** (Query Callers) - `pkg/chain/ethereum/tbtc.go` `WalletReservations` (cheap `WalletReservationsCount` short-circuit at line 1317) and `WalletTerminationCause` use the cached registration block as `startBlock`; `resolveCustodiedReservationKeys` skips unreadable individual reservation keys with a warning rather than aborting the whole lookup.

**If violated** - Without the bound, every wallet reservation query would scan full chain history from genesis, overwhelming Ethereum RPC providers; with the fail-pass behavior, a broken provider degrades liveness (no new notifications) rather than unbounded RPC load.

---

## 4. Economic Invariants

Higher-order economic properties derived from single-component and cross-component invariants.

---

#### E-1

Enforced: **Yes**

> Minted tBTC in the `ReservationVault` is 100% backed by Bitcoin satoshis held in the anchor UTXO; re-anchor Bitcoin network fees deducted from the anchor are tracked as `inKindFeeDebtSat` and funded via vault tBTC reserves or permissionless repayment, preserving systemic 1:1 backing (per spec).

**Follows from** - `I-2` + `I-7` + `X-2`

**If violated** - Re-anchor fee deductions without corresponding in-kind debt tracking or reserve financing would cause minted tBTC supply to exceed actual Bitcoin reserves held in custody.

---

#### E-2

Enforced: **Yes**

> For all wallet transactions, the fee is bounded below by the fixed protocol minimum `ProtocolMinWalletTxSatPerVByteFee * (txVsize - 1)` and bounded above by on-chain `txMaxFee` / `maxTotalFee`, on both the leader and every patched follower.

**Follows from** - `I-7` + `X-4`

**If violated** - Sub-floor fees would cause transactions to stick in Bitcoin mempools, while excessive fees would violate on-chain governance caps and fail SPV proof submission.

---

#### E-3

Enforced: **Yes** (new in this head)

> Re-anchor custody wind-down is constrained to targets that are Live, within the on-chain caps, and older than the wallet creation period as measured on chain time: custody of reserved deposits can only migrate to mature wallets, so a re-anchor can never concentrate reservation custody in a freshly created wallet.

**Follows from** - `I-9` (Live + caps) + target-age gate (`pkg/tbtc/reservation_target_age.go:20-28`, enforced at `pkg/tbtcpg/reservation_reanchor.go:368,1101` and `pkg/tbtc/reservation.go:901-916`)

**If violated** - Re-anchors into young wallets could pin new custody to wallets whose key material is immature or whose operators are unproven, and could race the wallet creation period's intent that fresh wallets absorb no custodial debt.

---

#### E-4

Enforced: **Yes**

> Reserved UTXOs and pooled wallet UTXOs remain segregated across every lifecycle transition: sweeps exclude reserved deposits (I-3/X-1), re-anchors move only anchor outputs between wallets (I-1), and the proof loop accepts only exact-shape 1-in-1-out candidates (I-1).

**Follows from** - `I-3` + `X-1` + `I-1`

**If violated** - Commingling reserved and unreserved UTXOs would allow standard redemptions to drain reserved depositor collateral, breaking deposit segregation guarantees.

# 08 - Threat model (M1 UTXO reservations)

Scope: M1 reservations only, both repos. Pinned commits: tbtc-v2 `reservations-upgrade` @ `9d2086965` (`tbtc-v2`), keep-core `reservations-epic` @ `f99ef96e3` (`keep-core`), Sepolia runbook `tbtc-v2` @ `52f4f3005`. The code is the source of truth; docs are background. D31 (privileged hand-off plus acceptance reserve) is integrated on the reservations branches at tbtc-v2 `reservations-upgrade` @ `806b1106f` and keep-core `reservations-epic` @ `d40f6aadc`; those facts carry an explicit revision in their citation, and an unqualified citation still means the original pin. Nothing is deployed on Sepolia or mainnet.

## How to read this file

- Every TM item gives: attacker, entry point (cited), preconditions, attack steps, impact, mitigations in code (cited), residual risk, test (or `none`), a "Negative stories" line, and a Status.
- Status legend: `holds` (code enforces the rule and a test pins it), `holds-untested` (code enforces it, no test found), `violated` (code breaks it: the path is given), `design-only` (planned, not in code), `unknown` (say what is missing to decide).
- Citations: `tbtc-v2:solidity/contracts/bridge/File.sol:line` for contracts, `tbtc-v2:solidity/test/bridge/File.test.ts:line` for tests; `keep-core:pkg/...:line` for the client; `tbtc-v2@52f4f3005:solidity/docs/RESERVATION_SEPOLIA_DEPLOYMENT.md:line` for the Sepolia runbook. Audit records are background only and are named without a pin.
- Assets covered: reserved BTC anchors, minted TBTC, the vault fee reserve, operator stake, wallet capacity slots, governance authority, and client availability.
- Abuse cases below are also the negative stories: each TM item names the user-story topics it frustrates from `04-user-stories.md` (cited by US label and topic title, e.g. US-71 "Hold cap exhausted (negative)"), under a "Negative stories" line.

## Summary matrix (threat x asset)

The table names assets exposed by each abuse case. Detailed entries state its preconditions and limits.

| ID | Threat | Exposed assets |
|---|---|---|
| TM-01 | Pending reveal griefing | Pending-deposit capacity; governance availability |
| TM-02 | Third-party target holds | Wallet capacity; custody liveness |
| TM-03 | Request front-running | Wallet capacity; transaction ordering |
| TM-04 | Generation replay | Anchor lineage; TBTC accounting |
| TM-05 | Forged or stale SPV proof | Bitcoin backing; credited TBTC |
| TM-06 | Fee grinding | Anchor value; vault reserve and fee debt |
| TM-07 | Forced stranding | In-kind status; pooled backing |
| TM-08 | Saturated retirement | Operator stake on the unproven path; custody liveness |
| TM-09 | Selector shadowing | Bridge authority and state; accounting |
| TM-10 | Storage collision | Reservation state; Bitcoin backing |
| TM-11 | Incorrect credit provenance | Routing promises; initiation fees; pooled-versus-reserved classification |
| TM-12 | Stale cleanup misuse | An eligible unaccepted intent, not a current Pending authorization |
| TM-13 | RPC failure or range limits | Client availability; proof and cleanup timing |
| TM-14 | Hostile parameter update | Capacity, signing liveness and fee economics |
| TM-15 | Incorrect activation order | Client coordination; uncredited funding |
| TM-16 | Withheld operator signatures | Custody liveness; possible operator penalties |
| TM-17 | Compromised client release | Wallet signatures and available custody controls |
| TM-18 | Compromised governance authority | Administrative controls; every asset dependent on them |
| TM-19 | Compromised vault owner | Fee revenue and reserve policy |
| TM-20 | Bitcoin reorg or fee manipulation | Proof finality assumptions; transaction liveness |

## Trust assumptions (stated, not re-derived)

- **Wallet signing assumptions.** The client distinguishes GroupSize, DKG GroupQuorum and signature HonestThreshold (`keep-core:pkg/tbtc/tbtc.go:28-45`). Signing requires at least the configured HonestThreshold ready members (`keep-core:pkg/tbtc/signing_loop.go:288`). Availability and malicious-key safety are separate assumptions. Do not use DKG quorum as the reservation signing threshold.
- **Relay difficulty is honest.** SPV proofs are checked against the Bridge relay's `getCurrentEpochDifficulty`/`getPrevEpochDifficulty` (`tbtc-v2:solidity/contracts/bridge/BitcoinTx.sol:300`); the relay is trusted not to understate difficulty. Proof work is gated by `observedDiff >= requestedDiff * txProofDifficultyFactor` (`tbtc-v2:solidity/contracts/bridge/BitcoinTx.sol:325`).
- **Governance is trusted, and parameter changes are delayed.** `updateReservationParameters` and `updateReservationCaps` run through a begin/finalize ceremony with a governance delay (`tbtc-v2:solidity/contracts/bridge/BridgeGovernance.sol:1834`, `:1893`, at `9d2086965`). At `806b1106f` the caps ceremony takes five arguments, the fifth being the hand-off reserve (`tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1896`, `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1917`); it is still delayed, and a zero reserve is rejected ("Hand-off reserve must be greater than zero", `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1701`). `setVaultStatus`, `setSpvMaintainerStatus`, `setReservationRouter`, and `forceStaleReservedDeposit` are `onlyOwner` with no delay (`BridgeGovernance.sol:308`, `:317`, `:1811`; `forceStaleReservedDeposit` at `tbtc-v2@c8350d109:solidity/contracts/bridge/BridgeGovernance.sol:1939-1941`): instant, trusted-governance-only. `forceStaleReservedDeposit` still requires no pending acceptance authorization, so it cannot kill a live acceptance; it does not check the refund deadline and is the override for a depositor who parked a far-future deadline.
- **Trusted protocol contracts.** Bank, the token vault, token and relay must implement their stated accounting and proof roles. Bank-only credit is not, by itself, reservation provenance: ordinary proved sweep credits also exist. VF-02 records that boundary.

## Adversary capability notes

- A **malicious depositor** controls the reveal call and the `requestReservationAcceptance` caller identity (`msg.sender == deposit.depositor`, `tbtc-v2:solidity/contracts/bridge/Reservation.sol:511`); it cannot call the permissionless re-anchor with a member proof it does not hold.
- A **third-party griefer** is any EOA that is not the owner, not an SPV maintainer, and not a proven source-wallet member; it reserves capacity against a target wallet but is bounded by the per-target hold cap and a cooldown. At `806b1106f` its holds no longer block a privileged hand-off off a retiring source (TM-02).
- A **malicious/lazy operator subset** is a minority of the custodying wallet's signing keyset that withholds or underprices signatures on Bitcoin actions.
- A **compromised governance key** signs any `onlyOwner` setter, including the no-delay `setSpvMaintainerStatus`, `setVaultStatus`, and `forceStaleReservedDeposit`.
- A **compromised vault owner** signs `sweepFees`, `updateFeeReserveTarget`, `updateInitiationFee` on the vault (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:302`, `:278`, `:335`).
- A **Bitcoin miner** can reorg recent blocks or inflate fees; the SPV confirmation depth and snapshotted fee bound are the backstops.
- An **Ethereum front-runner** reorders same-block calls; the request-time capacity reservation and cooldown bound the damage.
- A **malicious RPC provider** can slow or cap `eth_getLogs`; client scans are chunked and bounded by activation.
- A **compromised keep-core release** can ship the wrong `reservationsActivationBlocks` entry or wrong cap mirrors.

---

## TM-01 Grief pending-reveal slots with a far-future refund locktime

Attacker: a malicious depositor. Entry point: `Deposit.revealDeposit` routed to the reservation vault (`tbtc-v2:solidity/contracts/bridge/Deposit.sol:215`). Preconditions: the reservation vault is set and trusted; the depositor controls the reveal.

Attack steps.
1. Reveal a reserved deposit (`vault = reservationVault`) choosing `reveal.refundLocktime` as far in the future as the contract allows.
2. Never fund the anchor and never call `requestReservationAcceptance`. The deposit parks a `pendingReservedDeposit` record and increments `pendingReservedDeposits` (`Deposit.sol:387`), which blocks `updateReservationParameters` from changing the vault (`Reservation.sol:1414`).

Impact: unresolved reveal records increase pending-deposit state and can block a vault change. Reveal does not reserve an active-position or wallet slot. The refund-deadline bound makes stale cleanup eligible later; a caller must submit it. Records do not self-clear.

Mitigations: reveal-time refund-deadline cap (`Deposit.sol:244`); permissionless stale cleanup `notifyStaleReservedDeposit` after the deadline (`Reservation.sol:1229`), which needs a caller; governance backstop `forceStaleReservedDeposit` (see TM-12).

Residual: the grief window is capped to one custody term plus a 24-hour margin (`tbtc-v2:solidity/contracts/bridge/WalletProposalValidatorConstants.sol:13`), and the per-deposit parking is one pending slot. No on-chain fee is charged to park a slot, so a bulk griefer spends gas but not funds.

Test: the refund-locktime cap and the pending marker are pinned indirectly by the acceptance guard test set (`tbtc-v2:solidity/test/bridge/Reservation.test.ts:206`, "guard 5: pendingReservedDeposit[key].isReserved"). A dedicated far-future-locktime parking test was not found.

Negative stories (04-user-stories.md, by topic): US-02 "Reserved reveal rejected: refund locktime beyond term + safety margin"; US-10 "Stale reserved deposit (permissionless) and force stale (governance)"
Status: `holds-untested`.

## TM-02 Grief a target wallet's capacity via third-party re-anchor holds

Attacker: a third-party griefer. Entry point: `requestReservationReanchor` with an empty member proof against a Live target, reachable while any source wallet is MovingFunds/Closing (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:866`). Preconditions: the source wallet is `MovingFunds` or `Closing` (the path is permissionless there by design, D-19); the target is Live.

Attack steps.
1. For each active reservation whose source wallet is already MovingFunds/Closing, call `requestReservationReanchor` with a chosen Live target and an empty `walletMembersIDs`.
2. Each call reserves the target's count and amount capacity at request time (`Reservation.sol:880`), so legitimate incoming reservations targeting that wallet revert with "Wallet reservations cap exceeded" (`Reservation.sol:887`) during the hold window.

Impact: temporary denial of capacity on a chosen target for up to `reservationActionTimeout`. This is the accepted risk recorded in the deploy doc ("Permissionless Re-Anchor Target Selection (Accepted Risk)" in `tbtc-v2-rehearsal` / `RESERVATION_CAPS_DEPLOYMENT.md`). A non-privileged hold still consumes the target's ordinary slot (D-19, `tbtc-v2:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:386`). At `9d2086965` that denial could also block a retiring source's hand-off. At `806b1106f` a privileged hand-off off a `MovingFunds` or `Closing` source skips the target's caps (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`), so the hold does not block it (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:553`). The denial still applies to third-party requests (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:466`) and to governance rotation off a Live source (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:485`).

Mitigations: the per-target third-party hold cap reverts a third-party request once `pendingThirdPartyReanchorHolds[target] >= maxThirdPartyReanchorHoldsPerWallet` (`Reservation.sol:869`; at `806b1106f`, `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:896-906`); the re-anchor cooldown blocks immediate re-request on the same reservation for every caller except governance (`Reservation.sol:760`; at `806b1106f`, `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:786-791`); privileged callers (owner, SPV maintainer, proven source-wallet member) are exempt from the hold cap and never count toward it (`Reservation.sol:786`).

Residual: Sepolia's hold cap is 1. The caller pays Ethereum gas but posts no dedicated bond. After the duration, an explicit timeout notification can release the hold (`Reservation.sol:963`). D31 is integrated at `806b1106f`: the retiring-source hand-off exemption is in code and tested, so the residual is third-party and Live-source requests against an occupied target, not the hand-off. A pending request on the same reservation key still blocks a new request on that key ("Reservation is not active", `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:783`). Not deployed on Sepolia or mainnet.

Test: cap blocks a third party at the limit (`tbtc-v2:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:133`), cap zero blocks entirely (`:250`), privileged bypass (`:151`, `:179`, `:204`), all at `9d2086965`. At `806b1106f`: a privileged hand-off skips the ordinary cap while a hold occupies the slot (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:553`), the member request succeeds after the hold times out (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:565`), a third party into a full target reverts (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:466`), and an invalid member proof cannot claim the exemption (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:235`). The mainnet fork run (a local mainnet-fork run (no pin)) exercised the third-party limits and a privileged above-cap hand-off.

Negative stories (04-user-stories.md, by topic): US-71 "Hold cap exhausted (negative)"; US-72 "Hold cap zero (negative)"; US-31 "Re-anchor: third party, owner, SPV maintainer, member proof exempt the third-party hold cap"; US-75 "Hold release on timeout / supersede / on-time settlement"
Status: `holds`.

## TM-03 Front-run an acceptance or a re-anchor (Ethereum MEV)

Attacker: an Ethereum front-runner. Entry point: same-block reordering of `requestReservationAcceptance` (depositor-only) or `requestReservationReanchor` (permissionless) (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:499`, `:742`). Preconditions: the actor observes a pending intent in the mempool.

Attack steps.
1. Observe a depositor's `requestReservationAcceptance` or a third-party re-anchor request before it mines.
2. Race it with a higher bid so the front-run lands first and the intended request reverts on "Acceptance already pending" (`Reservation.sol:537`) or the cooldown/hold-cap reverts.

Impact: the front-run does not move funds or mint; it can only reserve capacity first and push the intended caller to a later generation. Because capacity is reserved at request time (`reserveAcceptanceCapacity`, `Reservation.sol:1004`), a front-run cannot double-credit or double-mint; it reorders who occupies a slot.

Mitigations: acceptance is depositor-only and one generation per key (`Reservation.sol:511`, `:537`); re-anchor request-time reservation is atomic per generation; the cooldown and hold-cap (TM-02) bound repeated third-party pressure; re-anchor from a Live source requires governance (`Reservation.sol:798`), so a front-run cannot rotate a healthy wallet's anchor.

Residual: same-block reordering of a depositor's own acceptance is possible and costs the depositor the window; the deposit still holds and can re-request. No on-chain reordering protection (no within-block ordering guarantee) exists.

Test: the "Acceptance already pending" and single-generation guards are pinned (`tbtc-v2:solidity/test/bridge/Reservation.test.ts:298`, "guard 9"). No dedicated MEV reordering test.

Negative stories (04-user-stories.md, by topic): US-05 "Acceptance request: capacity and amount-cap gates (negative)"; US-25 "Re-anchor: cap revert -> next candidate"; US-31 "Re-anchor: third party, owner, SPV maintainer, member proof exempt the third-party hold cap"
Status: `holds-untested`.

## TM-04 Replay a proof against the wrong generation

Attacker: an SPV maintainer (malicious or buggy) replaying an already-settled or superseded generation. Entry point: `submitReservationReanchorProof` / `submitReservationAcceptanceProof` (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:289`, `:263`). Preconditions: a stale action record still exists.

Attack steps.
1. Capture the `(reservationKey, requestNonce)` of a settled, timed-out, or superseded generation.
2. Resubmit its SPV proof, hoping the state machine re-settles it.

Impact: a double-settle would move an anchor twice or mint twice. Prevented in code.

Mitigations: `loadSettleableAction` only accepts `Pending` or `TimedOut` records and the matching `actionType` (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:192`, `:196`); `consumeAnchor` requires the input to be the reservation's *current* anchor outpoint and marks it `spentMainUTXOs` (`ReservationProofs.sol:908`, `:922`); `requireCurrentSourceAnchor` reverts "Action source anchor is no longer current" when the reservation moved to a newer anchor (`ReservationProofs.sol:316`); on-time settlement requires `reservation.requestNonce == requestNonce` (`ReservationProofs.sol:665`). `consumeAcceptedDeposit` requires `deposit.sweptAt == 0` (`ReservationProofs.sol:419`).

Residual: late settlement of a *timed-out* generation is still allowed (by design) to close an already-confirmed anchor; the spend-set and source-anchor binding make a second settle impossible. The window is not uniform: a late **acceptance** settles only while `block.timestamp <= timeoutAt + termSeconds` (`tbtc-v2@c8350d109:solidity/contracts/bridge/ReservationProofs.sol:198-211`, "Late acceptance settlement window expired"), while late **re-anchor** settlement carries no equivalent bound. Acceptance after the cutoff is not a replay, it is a lost settlement (AR-10, TM-16).

Test: a timed-out re-anchor replay against a later anchor reverts (`tbtc-v2:solidity/test/bridge/Bridge.ReservationSourceAnchorBinding.test.ts:412`).

Negative stories (04-user-stories.md, by topic): US-09 "Superseding acceptance: late settlement of an older generation unwinds a newer pending generation"; US-44 "Action not settleable (negative)"; US-33 "Re-anchor SPV proof settlement"
Status: `holds`.

## TM-05 Forge or replay a stale SPV proof

Attacker: a malicious SPV maintainer. Entry point: the two proof functions, gated by `onlySpvMaintainer` (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:195`). Preconditions: the attacker is on the `isSpvMaintainer` allow-list.

Attack steps.
1. Submit a proof that does not actually prove the claimed Bitcoin transaction (forged header chain, wrong merkle root, or a stale/forked block).
2. Aim to mint TBTC for an anchor that was never confirmed on the real chain.

Impact: unbacked TBTC mint.

Mitigations: `validateProof` checks the input/output vectors, the transaction merkle inclusion, the coinbase merkle inclusion, and the header-chain difficulty against the relay's current/previous epoch difficulty, gated by `txProofDifficultyFactor` (`tbtc-v2:solidity/contracts/bridge/BitcoinTx.sol:188`); `evaluateProofDifficulty` reverts "Insufficient accumulated difficulty in header chain" when work is below the relay's epoch (`tbtc-v2:solidity/contracts/bridge/BitcoinTx.sol:325`). Reorg protection comes from the confirmation depth: the SPV maintainer scans to tip-minus-12 and requires `requiredConfirmations` before building a proof (`keep-core:pkg/maintainer/spv/reservation_event_scan.go:27`).

Residual: the relay difficulty oracle is trusted (Trust assumption); a maintainer cannot forge a valid-enough proof, but the *trust* in the relay is the boundary. Compromising the relay to understate difficulty reopens this.

Test: the shared difficulty/merkle negatives are pinned for the sweep path (`tbtc-v2:solidity/test/bridge/Bridge.Deposit.test.ts:4223`, "Insufficient work in a header"; `:4282`). The reservation proof suite drives the same `validateProof` (`tbtc-v2:solidity/test/bridge/ReservationProofs.test.ts`, relay mock at `:155`).

Negative stories (04-user-stories.md, by topic): US-42 "Proof submission gate (negative)"; US-43 "Wrong proof type (negative)"; US-45 "Anchor / input / output reverts in the proof (negative)"
Status: `holds`.

## TM-06 Grind fees across re-anchor hops

Attacker: a malicious operator or an owner pushing re-anchors. Entry point: `requestReservationReanchor` / `submitReservationReanchorProof` (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:742`, `tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:640`). Preconditions: a reservation is Active and re-anchoring is permitted.

Attack steps.
1. Re-anchor repeatedly, each hop spending up to `reservationTxMaxFee` in miner fees deducted in-kind from the anchor.
2. Wind the anchor down toward the dust floor to squeeze value.

Impact: the anchor's in-kind value erodes; `mintedAmount` is written down on each hop (`ReservationProofs.sol:818`).

Mitigations: request-time floor `anchorAmount > txMaxFee + minAmount` caps the cumulative grind at `(initial anchor - minAmount)` and reverts "Reanchor would fall below the minimum reservation amount" (`Reservation.sol:854`); per-hop `action.amount - newAnchorAmount <= action.txMaxFee` and dust floor `newAnchorAmount > action.txMaxFee` (`ReservationProofs.sol:687`, `:695`); lifetime total accumulates in `cumulativeReanchorFee` for a post-M1 cap (`ReservationProofs.sol:810`). The keep-core re-anchor task mirrors the floor and cooldown (`keep-core:pkg/tbtcpg/reservation_reanchor.go:470`).

Residual: no per-hop cumulative ceiling is enforced in M1; the request-time amount floor is the only on-chain bound, and M1 ships without a `maxCumulativeReanchorFee`.

Test: the request-time floor and the no-Live-wallet-rotation gate are pinned (`tbtc-v2:solidity/test/bridge/Bridge.ReservationSettlement.test.ts:500`, `:587`).

Negative stories (04-user-stories.md, by topic): US-27 "Re-anchor: aged target, cooldown, request-time floor"; US-33 "Re-anchor SPV proof settlement"; US-69 "Debt repayment and in-kind fee financing"; PAR-X8 "Re-anchor min amount floor vs settle dust floor" in 03-parameters-governance.md
Status: `holds`.

## TM-07 Force stranding to dump a shortfall on TBTC holders

The published code and the intended policy agree: a position on a healthy (Live)
wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was
taken on 2026-10-29 and the code does not change.

Attacker: a griefer driving a custodying wallet to a state where its reservation can no longer move, so it is stranded and the shortfall is absorbed by all TBTC holders. Entry point: `notifyReservationStranded` (permissionless), reachable when the wallet is Terminated or Closed, when it is a Live wallet past the reservation's `dissolutionEligibleAt`, or when it is Closing past both `dissolutionEligibleAt` and the closing period (`tbtc-v2@c8350d109:solidity/contracts/bridge/Reservation.sol:1325-1336`). Being Live is therefore no longer a precondition against stranding; the deadline is.

Attack steps.
1. Via TM-02/TM-08 pressure, keep the wallet's reservation count non-zero so it cannot close cleanly.
2. Let the moving-funds clock expire (the wallet is Terminated, slashing operators), let a Closing wallet pass `dissolutionEligibleAt` and its closing period, or wait out the position's own `dissolutionEligibleAt` on a wallet that stays Live.
3. Call `notifyReservationStranded`; `strandReservation` releases capacity and emits `ReservationStranded` (`Reservation.sol:1101`); the anchor is not recovered.

Impact: the owner keeps nominal TBTC but the in-kind claim is lost; the anchor's shortfall is shared by every TBTC holder. There is no M1 compensation; the event is evidence only (the audit record `m1-integrator-due-diligence.md`, D6 row).

Mitigations: stranding only fires on one of the four admissible states, and the Closing branch adds a full closing period of re-anchor time (`tbtc-v2@c8350d109:solidity/contracts/bridge/Reservation.sol:1326-1336`); a pending action cannot be stranded, so a timed-out but signed re-anchor can still settle late and restore the position (`ReservationProofs.sol:297`); capacity release on strand keeps the system from deadlocking.

Residual: this is the accepted M1 loss story; no on-chain compensation exists. The attack is a *forcing* path, not a forgery; it costs the griefer only griefing pressure.

Test: stranding transition and counter decrement are pinned (`tbtc-v2:solidity/test/bridge/Bridge.ReservationStranding.test.ts:525`, `tbtc-v2:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:110`). The cross-cutting chain (pressure -> slash -> strand) is not rehearsed end-to-end.

Negative stories (04-user-stories.md, by topic): US-13 "Stranding on a Terminated wallet"; US-14 "Stranding on a Closing wallet after dissolution eligibility and the closing period"; US-101 "Stranding on a dissolution-eligible Live wallet"; US-49 "Stranding watcher (keep-core side)"
Status: `holds-untested`. The entry conditions above replace the earlier "the wallet has left Live" framing. The client-side gap (VF-10) was the shipped watcher never taking the Live branch; keep-core #4365 added it, and the branch runs on the startup catch-up scan and the action-timeout recheck queue.

## TM-08 Slash honest operators via saturation

Attacker: a third-party griefer exhausting `maxActiveReservations` and per-wallet slots so that a retiring wallet cannot re-anchor. Entry point: `requestReservationReanchor` (TM-02) plus the wallet-closing gate (`tbtc-v2:solidity/contracts/bridge/Wallets.sol:384`). Preconditions: live-wallet slot supply is low relative to `maxActiveReservations`. At `806b1106f` full target caps no longer stop a privileged hand-off, so this attack now needs more than slot pressure: no Live target, a signing keyset below quorum, or an undelivered proof.

Attack steps.
1. Fill global occupancy and per-wallet capacity so no Live target accepts a re-anchor (at `9d2086965`: every re-anchor; at `806b1106f`: every third-party or Live-source re-anchor, while a privileged hand-off off a retiring source still succeeds, `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:427`).
2. A custodying wallet that wants to retire is forced into MovingFunds (`tbtc-v2@c8350d109:solidity/contracts/bridge/Wallets.sol:629-631`); with a proven reservation anchor it cannot reach Closing, and the final close needs a zero reserved count (`tbtc-v2@c8350d109:solidity/contracts/bridge/Wallets.sol:392-395`). Capacity that is only reserved for pending actions does not hold the wallet in MovingFunds (VF-11), so the pressure has to reach proven custody to keep the retirement path open.
3. The moving-funds clock expires; the timeout path slashes operator stake and Terminates the wallet, stranding the anchor (TM-07).

Impact: honest operators lose stake; the reservation is stranded.

Mitigations: `maxActiveReservations` is sized *below* the free-slot supply so acceptance reverts before the occupancy cliff (`tbtc-v2:solidity/contracts/bridge/BridgeState.sol:435`); the launch gate `maxActiveReservations > 0` is enforced (`Reservation.sol:1501`); the slot-capacity invariant keeps `reservationTotalAmount` bounded to slot capacity (`Reservation.sol:1322`); stranding releases capacity so the system self-heals as wallets retire. The Sepolia values keep `maxActive` at 2 of 4 slots to leave room for handoffs (`tbtc-v2@52f4f3005:solidity/docs/RESERVATION_SEPOLIA_DEPLOYMENT.md:162`). At `806b1106f` two further controls exist. A privileged hand-off off a non-Live source skips the target caps (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:921-922`). The acceptance reserve rejects a new acceptance once the active count passes `(liveWalletsCount - handoffReserveWallets) * maxReservationsPerWallet` ("Hand-off reserve exceeded", `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1076`; the check is `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1071-1077`). The Sepolia design values are a reserve of 2 with four Live wallets and one slot per wallet, so two admissions fit (see [approved design deltas](13-approved-design-deltas.md)); nothing is deployed there.

Residual: the reserve is a count bound at request time only. It does not guarantee a Live target, a signing quorum, proof delivery, amount headroom or fee room, and hand-offs above the cap concentrate anchors on one wallet. Sizing is still a governance discipline: a reserve at or above the Live wallet count rejects every acceptance, and a reserve of zero is rejected by the setter. At `9d2086965` the guard was sizing only and the cliff reopened if caps were sized above live slot supply.

Test: the moving-funds and closing gates with reservations are pinned (`tbtc-v2:solidity/test/bridge/Reservation.test.ts:1813`, at `9d2086965`). At `806b1106f` the gate is pinned by `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:322` (rejects past `(L - R) * N` with max-active room), `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:346` (tightens when a Live wallet leaves Live) and `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:379` (blocks every acceptance while `L <= R`). The pressure -> slash -> strand chain is not rehearsed end-to-end.

Negative stories (04-user-stories.md, by topic): US-05 "Acceptance request: capacity and amount-cap gates (negative)"; US-06 "Acceptance timeout and re-authorization"; US-61 "begin/finalize staging for caps"
Status: `holds`.

## TM-09 Shadow a selector through the delegatecall router

Attacker: an off-chain actor sending a call to the Bridge whose selector is meant to hit the router but shadows a Bridge method (or vice versa), expecting different behavior on the shared Bridge storage. Entry point: `Bridge.fallback` delegatecalls unmatched selectors to `reservationRouter` (`tbtc-v2:solidity/contracts/bridge/Bridge.sol:2119`). Preconditions: the router is set.

Attack steps.
1. Send a selector that the Bridge does not declare; it is delegatecalled to the router code running on Bridge storage.
2. If the router re-declared a Bridge selector with different behavior, two code paths would collide on one storage.

Impact: a shadowing selector with divergent behavior corrupts Bridge state; a stand-alone call to the router address (not via the Bridge) executes on empty storage and reverts (the router has no authority of its own, Router invariant 3, `tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:82`).

Mitigations: the router declares the *only* storage variable `BridgeState.Storage internal self` so storage slots match (`ReservationRouter.sol:100`); a selector-disjointness guard is pinned and the only documented exception is the shared `Governable`/`Initializable` base, whose router-side copies are permanently unreachable because the Bridge's own copy answers first (`ReservationRouter.sol:74`); calling the router directly reverts because `governance` is unset, no maintainer is approved, and the vault is zero (`ReservationRouter.sol:82`).

Residual: correctness depends on the disjointness guard holding for every future router method; a new shadowing method must keep the test green.

Test: `tbtc-v2:solidity/test/bridge/Bridge.RouterSelectorDisjointness.test.ts:53` (shares no 4-byte selector) and `Bridge.RouterStorageParity.test.ts:196`, `:226` (same slot and same member layout).

Negative stories (04-user-stories.md, by topic): No US story: this is a system-model invariant, "ReservationRouter invariants" in 01-system-model.md
Status: `holds`.

## TM-10 Collide storage on a Bridge upgrade

Attacker: none external; this is a self-defect risk from a later Bridge implementation writing into a slot the router or reservation libraries read. Entry point: any future `BridgeState.Storage` edit (`tbtc-v2:solidity/contracts/bridge/BridgeState.sol:505`). Preconditions: a new Bridge upgrade.

Attack steps.
1. A new BridgeState field is inserted mid-struct or the `__gap` is spent carelessly, shifting slots under `reservations`, `reservationActions`, or the packed slot 33.
2. The router, which mirrors Bridge storage, now reads a shifted slot.

Impact: silent reservation-state corruption on upgrade.

Mitigations: reservation state is append-only: new `ReservationAction` fields (`minAmount`, `thirdPartyHold`) and `ReservationRequest` fields (`cumulativeReanchorFee`, `reanchorCooldownUntil`) are appended to the struct end with a documented no-gap note because the struct is stored in a mapping (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:234`, `:350`); a `uint256[37] __gap` budget is reserved with an explicit instruction not to assume it is spendable by unrelated PRs (`BridgeState.sol:505`); a storage-layout test diffs the router against the Bridge and against the checked-in snapshot.

Residual: the snapshot test only protects against drift from the *pinned* layout; a reviewer must keep the parity test green on every Bridge change.

Test: `tbtc-v2:solidity/test/bridge/Bridge.StorageLayout.test.ts:45` ("upgrade-safe against the deployed TIP-109 Bridge layout"), `:182` ("matches the checked-in ... snapshot layout").

Negative stories (04-user-stories.md, by topic): US-63 "setReservationRouter (one-shot)"
Status: `holds`.

## TM-11 Mint TBTC for an unverified credit

Attacker: a compromised or buggy governance path that marks a vault trusted and then routes a non-reservation credit to it. Entry point: `ReservationVault.receiveBalanceIncrease` (called by Bank) and `setVaultStatus` (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:161`, `tbtc-v2:solidity/contracts/bridge/BridgeGovernance.sol:308`). Preconditions: a vault is marked trusted.

Attack steps.
1. `setVaultStatus(vault, true)` with no reservation proof having preceded it.
2. Bank credits the vault via `increaseBalanceAndCall` (an ordinary pooled sweep or a mis-routed credit); the vault mints TBTC gross (`ReservationVault.sol:178`) with no `reservationKey` to verify provenance.

Impact: pooled sweep credit can use reservation-specific fee/mint handling without a reservation key. The proved sweep still contributes BTC to the pool, and mint consumes Bank credit. This does not demonstrate arbitrary unbacked supply.

Mitigations: the reservation credit path is the SPV-proven acceptance (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:621` routes through `deposit.vault` only when `isVaultTrusted`, else credits the depositor directly with no fee); a vault re-point is gated on `reservationTotalAmount == 0 && pendingReservedDeposits == 0` (`Reservation.sol:1409`); the deploy script wires trust only after Bridge wiring (`ReservationVault.sol:147`). The vault itself is `Ownable`, not upgradeable (Option B), so its credit logic cannot be patched in place.

Residual: the documented provenance gap remains relevant when governance trusts a vault that is not correctly wired. VF-02 confirms this narrower path. The known-gap comment is not a completed fix or an acceptance decision.

Test: `tbtc-v2:solidity/test/vault/ReservationVault.test.ts:137` pins the `onlyBank` guard and the gross-mint fee math; no test exercises a *non-reservation* credit arriving at a trusted vault.

Negative stories (04-user-stories.md, by topic): US-11 "Mint, fee, and the credit path"
Status: `holds-untested` (the gap is acknowledged; the no-verification residual is Finding F-04).

## TM-12 Kill a pending deposit via stale notify

Attacker: governance (misuse) or an over-eager permissionless caller. Entry point: `forceStaleReservedDeposit` (`onlyGovernance`) and `notifyStaleReservedDeposit` (permissionless) (`tbtc-v2:solidity/contracts/bridge/ReservationRouter.sol:410`, `:396`). Preconditions: a pending reserved deposit exists.

Attack steps.
1. Governance force-clears an unswept pending reveal with no current Pending acceptance authorization.
2. The pending routing/deadline record is removed. The reserved marker remains; this is not D26 release to pool.
3. If an acceptance is currently Pending, the proposed call reverts with "Acceptance authorization pending".

Impact: a legitimate pending reservation intent is discarded; the owner loses the in-kind path for that deposit.

Mitigations: `forceStaleReservedDeposit` still requires no pending acceptance authorization and `sweptAt == 0` (`Reservation.sol:1300`); it is `onlyGovernance` and exists to release a griefing depositor who parked a far-future deadline (TM-01); `notifyStaleReservedDeposit` is permissionless but gated by the exact snapshotted refund deadline and by no pending acceptance (`Reservation.sol:1248`).

Residual: governance can discard an eligible unrequested or timed-out intent without waiting for the depositor's refund deadline. This is an intentional anti-grief power. It cannot clear a current Pending acceptance.

Test: `notifyStaleReservedDeposit` is pinned (`tbtc-v2:solidity/test/bridge/Bridge.ReservationStranding.test.ts:727`, "succeeds once the deadline has elapsed"). `forceStaleReservedDeposit` has no dedicated test.

Negative stories (04-user-stories.md, by topic): US-10 "Stale reserved deposit (permissionless) and force stale (governance)"; US-65 "forceStaleReservedDeposit (governance override)"
Status: `holds-untested` (notify holds and is tested; force is untested, see Findings).

## TM-13 DoS keep-core event scans via an unbounded scan or a hostile RPC provider

Attacker: a malicious RPC provider (throttles `eth_getLogs`) or, indirectly, chain load. Entry point: keep-core's SPV reservation watchers and the re-anchor target search (`keep-core:pkg/maintainer/spv/reservation_event_scan.go`, `keep-core:pkg/tbtcpg/reservation_reanchor.go`). Preconditions: the client is running reservation duties.

Attack steps.
1. A provider with a low `eth_getLogs` cap is fed a catch-up scan; chunked fetches (10,000 blocks) progress but slowly, delaying proofs, timeouts, and stranding.
2. A full-history fallback can fail when the RPC restricts query ranges.

Impact: necessary chain reads and cleanup can be delayed.
Unspent funding retains its refund path after locktime.
Confirmed acceptance spends need proof recovery, not a refund.
No attacker-amplified workload was demonstrated in this review.

Mitigations: all hot-path scans are activation-block-bounded, chunked, and stop 12 blocks behind the tip (`keep-core:pkg/maintainer/spv/reservation_event_scan.go:19`); candidate discovery is capped (`maxReservationAcceptanceCandidatesPerRun = 50`, `keep-core:pkg/tbtcpg/reservation_acceptance.go:120`); per-tick watcher budgets (`keep-core:pkg/maintainer/spv/reservation_action_timeout_watch.go:37`); the re-anchor target search is bounded to `ReservationReanchorLookBackBlocks = 216000` blocks (`keep-core:pkg/tbtcpg/reservation_reanchor.go:21`).

D28 adds historical wallet discovery outside the reservation activation boundary, still in bounded chunks
(`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:48-82`).
Required observation reads must succeed before risk values or the timestamp advance
(`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_observer.go:98-190`).
The monitor reports an unavailable endpoint or excessive timestamp deviation as Critical
(`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:106-115`,
`tbtc-v2@8eb26c2e2:monitoring/src/reservation-keeper-monitor.ts:192-225`).
Numeric validation and freshness do not authenticate the metrics source or an RPC provider's chain data.
Private endpoint access, the matching network and a trusted incident route remain operator prerequisites
(`keeper-drill-setup.md:81-83`,
`keeper-drill-setup.md:110-143`).
An alert improves detection; it does not remove the RPC trust assumption or guarantee timely proofs.

Residual: target discovery and the below-dust history check include full-history queries. Repeated passes may repeat them. The provider compatibility risk is real; VF-05 rejected the stronger attacker-DoS claim.

Test: none for RPC-provider throttling; the bounded-scan behavior is unit-tested (`keep-core:pkg/maintainer/spv/reservation_event_scan_test.go`).

Negative stories (04-user-stories.md, by topic): US-28 "Activation-block gating: client and on-chain coupling"; US-29 "Deposit sweep rejects reserved deposits and fails closed on reservation parameter error"; US-46 "Per-item errors do not block sibling actions; restart resumes scan"; US-51 "Watcher restart and metrics"
Status: `holds-untested` (bounded paths hold; unbounded residuals are Finding F-03).

## TM-14 Abuse a parameter governance write

Attacker: a compromised or malevolent governance key. Entry point: `updateReservationParameters` / `updateReservationCaps` via the begin/finalize ceremony (`tbtc-v2:solidity/contracts/bridge/BridgeGovernance.sol:1834`, `:1893`, at `9d2086965`; at `806b1106f` the caps entry is `beginReservationCapsUpdate` with five arguments, `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1896`, finalized at `tbtc-v2@806b1106f:solidity/contracts/bridge/BridgeGovernance.sol:1917`). Preconditions: governance signs.

Attack steps.
1. Stage caps that are *feasible* but hostile: raise `reservationTxMaxFee` to inflate per-hop fees, drop `reservationMaxSingleAmount` to shrink usable capacity, or set `maxReservationsPerWallet` to 1 to squeeze per-wallet headroom.
2. The on-chain guard only blocks the "zero feasible deposits" state, not a feasible-but-hostile setting.

Impact: operational griefing of depositors and re-anchor economics. At `806b1106f` a hostile fifth caps argument is a new lever: a reserve at or above the Live wallet count rejects every new acceptance (`tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:379`); it takes effect only after the governance delay, and the setter rejects zero (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1701`). It does not move funds or block a privileged hand-off, which does not read the reserve.

Mitigations: the feasibility guard `assertReservedDepositsFeasible` runs on both reservation setters and on `updateDepositParameters` while a vault is set (`tbtc-v2:solidity/contracts/bridge/BridgeState.sol:642`, `Reservation.sol:1426`), blocking a cap below the dust threshold and a reveal-ahead period that leaves no reveal window (`BridgeState.sol:681`); the slot-capacity invariant `reservationMaxTotalAmount <= maxActiveReservations * reservationMaxSingleAmount` is enforced on both setters (`Reservation.sol:1322`); the launch gate `maxActiveReservations > 0` is a `require` (`Reservation.sol:1501`). The keep-core maintainer mirrors the feasibility math and raises `reservation_params_infeasible` as an alarm (`keep-core:pkg/maintainer/spv/reservation_params_feasibility.go:37`; gauge `keep-core:pkg/clientinfo/performance.go:797`).

Residual: the no-delay `onlyOwner` setters `setVaultStatus` and `setSpvMaintainerStatus` (`BridgeGovernance.sol:308`, `:317`) let a compromised key revoke vault trust or approve a malicious SPV maintainer instantly (see TM-18).

Test: the feasibility and slot-capacity reverts are pinned (`tbtc-v2:solidity/test/bridge/Bridge.ReservationFeasibilityGuard.test.ts:170`, `:192`, `:235`; `Bridge.ReservationCaps.test.ts:95`).

Negative stories (04-user-stories.md, by topic): US-60 "begin/finalize staging for parameters"; US-61 "begin/finalize staging for caps"; US-66 "updateDepositParameters feasibility guard (D25)"; US-64 "Revert when finalize runs before the delay elapses or the change is uninitialized"
Status: `holds` (the guard holds and is tested); the no-delay residual is tracked in TM-18 / Finding F-02.

## TM-15 Activate on-chain before clients ship (C-4)

Attacker: an ordering mistake by governance (or a compromised key that runs the wrong step). Entry point: `setVaultStatus(vault, true)` vs the keep-core activation block (`tbtc-v2:solidity/contracts/bridge/BridgeGovernance.sol:308` vs `keep-core:pkg/tbtc/coordination.go:110`). Preconditions: the client release is not yet on every node.

Attack steps.
1. Governance executes `setVaultStatus(vault, true)` at a block *before* keep-core's activation block B.
2. Reserved reveals and acceptance requests are accepted on-chain, but no keep-core schedules anchor/re-anchor until B; pending acceptances time out and depositors take the Bitcoin refund, or a wallet signs out-of-band.

Impact: funding may wait without client coordination. Unspent funding can use a refund after locktime or eligible D26 release before any request. An already-spent output cannot refund. An unconfigured public-network client remains inactive until a corrected build provides an entry.

Mitigations: `reservationsActivationBlocks` ships empty and defaults non-Developer networks (including `Unknown`) to `math.MaxUint64`, i.e. never activate, instead of a silent block-0 (`keep-core:pkg/tbtc/coordination.go:110`); the reserved-deposit sweep gate derives `reservationsActive` from the block comparison, not from RPC success (`keep-core:pkg/tbtc/deposit_sweep.go:233`); the Sepolia runbook makes "ship the client with B compiled in, then set B at or after the release" an explicit C-4 step (`tbtc-v2@52f4f3005:solidity/docs/RESERVATION_SEPOLIA_DEPLOYMENT.md:715`).

Residual: the coupling is *runbook discipline; nothing on-chain enforces it* (deploy doc "Client Activation Ordering Gate"). A governance step executed out of order is not blocked.

Test: keep-core pins `ReservationsActivationBlock(Mainnet/Unknown) == MaxUint64` (`keep-core:pkg/tbtc/coordination_test.go:1608`). No on-chain guard exists.

Negative stories (04-user-stories.md, by topic): US-28 "Activation-block gating: client and on-chain coupling"; US-62 "setVaultStatus (vault activation / deactivation, C-4 gate)"
Status: `design-only` on the on-chain side (no code enforces the ordering); the client side is `holds-untested`.

## TM-16 Withheld or bad operator signatures (malicious / lazy wallet subset)

Attacker: a minority of the custodying wallet's signing keyset (malicious or just lazy). Entry point: the threshold-signing loop for anchor and re-anchor Bitcoin transactions (`keep-core:pkg/tbtc/signing_loop.go:288`). Preconditions: the wallet is custodying a reservation.

Attack steps.
1. Up to `groupSize - HonestThreshold` signers withhold, or the leader proposes a below-minimum fee.
2. The action misses its signing window; on-chain it times out (`ReservationActionTimeout`), the target capacity releases, and the re-anchor cooldown starts (`tbtc-v2:solidity/contracts/bridge/Reservation.sol:963`).

Impact: a requested anchor/re-anchor is not completed in time; the generation times out. A late confirmed anchor **re-anchor** still settles late (TM-04 path) with no time bound. A late **acceptance** does not: it settles only while `block.timestamp <= timeoutAt + termSeconds` (`tbtc-v2@c8350d109:solidity/contracts/bridge/ReservationProofs.sol:198-211`, "Late acceptance settlement window expired", negative test `tbtc-v2@c8350d109:solidity/test/bridge/ReservationProofs.test.ts:446`). Past that cutoff the confirmed anchor is not credited by this path; the position stays as it is and the deposit's in-kind claim is not restored (AR-10). The unconditional "so no funds are lost" reading is withdrawn (VF-13).

Mitigations: signing completes at `HonestThreshold` so a sub-majority cannot block (`keep-core:pkg/tbtc/signing_loop.go:288`); followers hard-reject below-minimum fees (`keep-core:pkg/tbtc/reservation.go:716`); on-chain the timeout + cooldown is the backstop, and a timed-out signed transaction remains provable within the window that applies to its action type (`tbtc-v2@c8350d109:solidity/contracts/bridge/ReservationProofs.sol:198-211`); a signed-but-unbroadcast transaction has a bounded re-attempt window (`keep-core:pkg/tbtc/reservation.go:513`). The D28 late-proof gauges exist to keep that window visible before it closes (`keep-core@9c4e1b221:pkg/maintainer/spv/reservation_keeper_risk.go:85-106`).

Residual: a stale or malicious *leader* can delay proposals; this is bounded by `ReservationActionTimeout`, an accepted tradeoff (`keep-core:pkg/tbtc/coordination.go:719`).

Test: none for a specific withholding minority; the fee-floor reject and the on-chain timeout path are the observable effects.

Negative stories (04-user-stories.md, by topic): US-20 "Acceptance proposal: discover, sign, broadcast"; US-22 "Acceptance task: candidate inside timeout safety margin"; US-23/US-24 re-anchor proposal paths
Status: `holds-untested` for the withholding defenses; the unconditional no-loss subclaim recorded before VF-13 does not match the published target and is replaced by the bounded statement above, which is the same limit AR-10 records.

## TM-17 Ship a compromised keep-core release

Attacker: a compromised build/release pipeline for keep-core. Entry point: the binary's `reservationsActivationBlocks` table and its cap mirrors (`keep-core:pkg/tbtc/coordination.go:92`, `keep-core:pkg/tbtcpg/reservation_reanchor.go:470`). Preconditions: a bad binary is deployed to operators.

Attack steps.
1. A compromised release ships a wrong activation block B (live too early = TM-15) or a wrong cap mirror that disagrees with the on-chain caps.
2. If a binary diverges from the on-chain activation block or cap values, coordinators and maintainers act on stale data.

Impact: mis-timed activation, missed proofs, or proposals rejected against on-chain caps.

Mitigations: the activation block is the single source of truth and every sweep/proposal decision is derived from the block comparison, not from RPC success (`keep-core:pkg/tbtc/coordination.go:105`); the re-anchor target search mirrors the on-chain caps live and treats cap reverts as "try the next target" rather than a failure (`keep-core:pkg/tbtcpg/reservation_reanchor.go:614`); the maintainer gauge `reservation_params_infeasible` surfaces a mismatched parameter state (`keep-core:pkg/clientinfo/performance.go:797`).

Residual: there is no on-chain proof that a running client's table is correct; a compromised release is detected only by divergence against the on-chain readouts.

Test: none against a poisoned binary; the derivation invariants are unit-tested (`keep-core:pkg/tbtc/coordination_test.go:1608`).

Negative stories (04-user-stories.md, by topic): US-51 "Watcher restart and metrics"; US-50 "Feasibility gauge (D25)"; US-46 "Per-item errors do not block sibling actions; restart resumes scan"
Status: `unknown` (out of scope of the two repos; depends on release integrity controls not in code).

## TM-18 Compromised governance key

Attacker: a compromised Bridge owner (Council Safe / Timelock on mainnet, deployer EOA on Sepolia). Entry point: the no-delay `onlyOwner` setters `setSpvMaintainerStatus`, `setVaultStatus` and `setReservationRouter`, plus the immediate `forceStaleReservedDeposit` (`tbtc-v2:solidity/contracts/bridge/BridgeGovernance.sol:317`, `:308`, `:1811`, `:1941`). Preconditions: the key is compromised.

Attack steps.
1. `setSpvMaintainerStatus(malicious, true)` instantly approves a malicious SPV maintainer (no governance delay).
2. `setVaultStatus(vault, false)` instantly revokes the reveal-time vault's trust; in-flight acceptances that then settle take the direct-credit fallback and lose the initiation fee.

Impact: a malicious maintainer cannot forge SPV proofs (TM-05) but can DoS or delay reservation settlement; a revocation changes fee accounting mid-flight.

Mitigations: parameter and cap changes remain behind the begin/finalize delay (`BridgeGovernance.sol:1834`); a malicious maintainer is still bound by SPV proof verification (`tbtc-v2:solidity/contracts/bridge/BitcoinTx.sol:325`), so the worst case is settlement delay, not unbacked mint.

Residual: the three `onlyOwner` no-delay setters (`setVaultStatus`, `setSpvMaintainerStatus`, `setReservationRouter`) plus the immediate `forceStaleReservedDeposit` are the real trust boundary; a compromised key bypasses the delay that protects everything else.

Test: none (a compromise is out of test scope).

Negative stories (04-user-stories.md, by topic): US-60 "begin/finalize staging for parameters"; US-61 "begin/finalize staging for caps"; US-62 "setVaultStatus"; US-64 "Revert when finalize runs before the delay elapses or the change is uninitialized"; US-65 "forceStaleReservedDeposit (governance override)"; US-68 "Vault fee and reserve management"
Status: `unknown` (depends on governance custody controls, not in the two repos).

## TM-19 Compromised vault owner

Attacker: a compromised vault owner (`onlyOwner`). Entry point: `sweepFees`, `updateFeeReserveTarget`, `updateInitiationFee` (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:302`, `:278`, `:335`). Preconditions: the vault is a separate `Ownable` contract (Option B), not the Bridge owner.

Attack steps.
1. `updateFeeReserveTarget(0)` so `sweepFees` can move the vault's entire TBTC balance out, including TBTC needed to finance in-kind miner fees.
2. `sweepFees(recipient)` to drain it; the re-anchor fee path then records `inKindFeeDebtSat` instead of reverting (`ReservationVault.sol:215`).

Impact: the fee reserve is drained; the system runs over-supplied by `inKindFeeDebtSat` until repaid. Settled re-anchors never fail because of the reserve, so this is a value-transfer to the vault owner, not a break of settlement.

Mitigations: `updateInitiationFee` is capped at `MAX_FEE_BASIS_POINTS = 500` (`ReservationVault.sol:335`); the debt is public (`inKindFeeDebtSat`) and repayable by anyone (`repayInKindFeeDebt`, `ReservationVault.sol:241`); `sweepFees` repays outstanding debt before sweeping the surplus (`ReservationVault.sol:302`), so draining cannot hide accrued debt.

Residual: the vault owner can pull fee revenue that was earmarked for in-kind financing; the shortfall is borne by the system as visible over-supply.

Test: `tbtc-v2:solidity/test/vault/ReservationVault.test.ts:273` pins `financeInKindFee` and the debt accounting; the owner-drain residual is the documented Fee-Reserve-Target governance step (`tbtc-v2@52f4f3005:solidity/docs/RESERVATION_SEPOLIA_DEPLOYMENT.md:792`).

Negative stories (04-user-stories.md, by topic): US-68 "Vault fee and reserve management"; US-69 "Debt repayment and in-kind fee financing"; US-81 "Sweep excess to treasury"; US-83 "balance approval always reverts"
Status: `holds-untested`.

## TM-20 Bitcoin reorg or fee manipulation

Attacker: a Bitcoin miner. Entry point: the SPV confirmation depth used to settle anchors/re-anchors (`keep-core:pkg/maintainer/spv/reservation_event_scan.go:27`). Preconditions: a recent block can be orphaned.

Attack steps.
1. Build a competing chain and orphan the block that confirmed a reservation anchor or re-anchor.
2. Attempt to settle a proof from the orphaned block, or restate a fee.

Impact: settling against an orphaned block would mint/move on a reverted Bitcoin state.

Mitigations: the SPV scan stops 12 blocks behind the tip (`reservation_event_scan.go:27`), so a proof is only built from a confirmed-enough block; a maintainer's `requiredConfirmations` further gates it; the snapshotted `txMaxFee` bound means a restate cannot inflate the in-kind fee past the generation's bound (`tbtc-v2:solidity/contracts/bridge/ReservationProofs.sol:687`); reorgs are the same risk as ordinary tBTC.

Residual: the 12-block and confirmation-depth margins are the reorg backstop; a reorg deeper than that window requires the maintainer to drop the proof and rescan.

Test: none specific to reservation reorgs; the confirmation-depth behavior is the shared SPV path.

Negative stories (04-user-stories.md, by topic): US-28 "Activation-block gating: client and on-chain coupling" (scans stop 12 blocks behind tip); EC-12/EC-13 "Ethereum / Bitcoin reorg" in 05-edge-cases.md
Status: `holds-untested`.

---

## Findings

The draft claims received an adversarial pass.
VF-02 retains the narrow vault-credit provenance gap.
VF-03 rejects destruction of a current Pending acceptance.
VF-04 treats immediate owner operations as an authority/trust assumption.
VF-05 retains RPC history compatibility, not a demonstrated attacker amplification.
See [finding adjudication](12-finding-adjudication.md) for the evidence and dropped claims.

## Doc drift

Where an existing doc disagrees with the pinned code. Code wins.

| Doc (path:line) | What it says | What the code does |
|---|---|---|
| `m1-b-implementation.md:34`, `m1-b-implementation.md:162`, `m1-b-implementation.md:263` | The vault ships with redemption/renewal entry points behind pause flags, ready to unpause | No pause flags exist; `ReservationVault` is `Ownable` and minimal (`tbtc-v2:solidity/contracts/vault/ReservationVault.sol:46`); redemption/renewal are M2 via a new vault + migration (Option B). `m1-b-implementation.md:452` confirms the pause-flag design was rejected. |
| `README.md` (decision 2026-09-07) | `#1122` reverted `#1121` | Confirmed by code: no reserved-redemption/veto/renewal surface in the M1 router (`tbtc-v2:solidity/contracts/bridge/IReservationBridge.sol:26`). Not drift; a matching fact recorded for reviewers. |
| `tbtc-v2@52f4f3005:solidity/docs/RESERVATION_SEPOLIA_DEPLOYMENT.md:119` | `RESERVATION_MAX_ACTIVE=5` in the pinned mainnet-fork command | This is a mainnet-fork value, not the Sepolia staging value. The runbook explicitly says not to reuse it unchanged. Sepolia D30 stages max active 2. |
| `architecture.md` (pause-flag vault) | Vault has unpause flags | Not in code (as above). |

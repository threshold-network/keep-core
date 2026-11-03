# Generation, timing and cross-contract invariants

Evidence pins and vocabulary: [invariants index](02-invariants.md).

## 5. Nonce / generation monotonicity and replay protection

### INV-501 `requestNonce` is monotonic; each generation is uniquely `(key, nonce)`

- **Statement.** Every accepted request bumps the position's `requestNonce`;
  an action generation is identified by `(reservationKey, requestNonce)` and
  stored under `actionKey(key, nonce)`. A generation, once terminal
  (`Settled`/`Superseded`/`Vetoed`), can never be re-settled.
- **Enforcing code.** `requestNonce = ++...` on request (`Reservation.sol:623`
  acceptance, `:907` re-anchor); `actionKey` (`Reservation.sol:419-427`);
  `loadSettleableAction` accepts only `Pending | TimedOut` (`
  ReservationProofs.sol:193-198`), so `Settled`/`Superseded`/`Vetoed`
  generations revert.
- **Why.** Replaying a settled or superseded generation would double-move the
  anchor or double-credit.
- **Test.** `ReservationProofs.test.ts` "should revert if action state is
  Settled (2)" (:315), "should revert if action state is Superseded (5)"
  (:353), "should revert if action state is Vetoed (4)" (:334).
- **Status.** holds.

### INV-502 On-time settlement must name the current generation; late settlement names an older timed-out one

- **Statement.** An on-time proof requires the named `requestNonce` to equal
  the position's current `requestNonce`. A late proof may name an older
  timed-out generation. A generation newer than a pending one is unwound to
  `Superseded`, not settled. When several generations of one position can be
  settled by the same Bitcoin transaction, the generation the maintainer
  proves is the newest `Pending` one, which the client sorts first, submits
  alone for the key and claims on success
  (`keep-core@96530a787:pkg/maintainer/spv/reservation_proof_loop.go:875-888`,
  `:1193-1195`, `:1275-1277`), and the older `TimedOut` generation that this
  settlement leaves behind can then never settle, because the position's
  anchor has moved away from the source anchor that generation recorded
  (`tbtc-v2@b7aece4bf:solidity/contracts/bridge/ReservationProofs.sol:321-330`),
  which INV-503 and EC-14 already accept.
- **Enforcing code.** On-time current-generation check
  (`ReservationProofs.sol:482-487` acceptance, `:663-667` re-anchor);
  `late` is derived from `action.state == TimedOut` (`
  ReservationProofs.sol:198`); supersede-unwind on late settle of a newer
  pending generation (`ReservationProofs.sol:506-518` acceptance,
  `:700-726` re-anchor -> `unwindPendingAction` :832-881, `require
  pending.state == Pending` at :844-847, sets `Superseded` at :849).
- **Why.** Settling a superseded or out-of-order generation would corrupt the
  anchor lineage and the capacity accounting.
- **Test.** `ReservationProofs.test.ts` "should unwind pending newer
  acceptance generation when settling older generation late" (:920), "should
  unwind pending reanchor action" (:1072), "should revert if
  unwindPendingAction is called on non-pending action" (:1037).
- **Status.** holds.

### INV-503 A timed-out generation remains settleable only within its window; a re-anchored anchor cannot be replayed

- **Statement.** A `TimedOut` acceptance generation settles late only within
  `timeoutAt + termSeconds`; a re-anchor late settlement is unbounded in M1.
  A timed-out re-anchor generation cannot be replayed against a *later*
  anchor: `requireCurrentSourceAnchor` reverts unless the position still
  points at the generation's recorded source anchor.
- **Enforcing code.** Late-acceptance window bound
  (`ReservationProofs.sol:204-209`); `requireCurrentSourceAnchor`
  (`ReservationProofs.sol:311-320`, called at :670 for re-anchor).
- **Why.** Prevents a stale generation from settling against an anchor that
  has since moved.
- **Test.** `Bridge.ReservationSourceAnchorBinding.test.ts` "rejects a
  timed-out re-anchor replay against a later anchor" (:412).
- **Status.** holds.

---

## 6. Time

### INV-601 `timeoutAt` and the timeout notification boundary

- **Statement.** The action snapshots `timeoutAt = requestedAt + action duration`.
  Timeout notification passes its time guard at `now >= timeoutAt`.
  The action-type, Pending-state and position guards must also pass.
- **Enforcing code.** `Reservation.sol:566-568` and `:824-826` (timeout
  computation); `block.timestamp >= action.timeoutAt` at `:686-690`
  (acceptance) and `:957-961` (re-anchor).
- **Test.** `Bridge.ReservationStrandingLibrary.test.ts` "reverts when action
  has not timed out" (:1027).
- **Status.** holds.

### INV-602 Re-anchor cooldown

- **Statement.** A timed-out re-anchor sets
  `reanchorCooldownUntil = now + (timeoutAt - requestedAt)`.
  An ordinary request requires `now >= reanchorCooldownUntil`.
  A privileged request bypasses this check: the governance-discriminated request,
  the owner, an SPV maintainer and a caller with a proven source-wallet member proof.
  At `9d2086965` only the governance flag bypassed it.
- **Enforcing code.** `tbtc-v2@b7aece4bf:solidity/contracts/bridge/Reservation.sol:849-854` checks `isPrivileged`, built at `tbtc-v2@b7aece4bf:solidity/contracts/bridge/Reservation.sol:831-835`.
  `tbtc-v2@b7aece4bf:solidity/contracts/bridge/Reservation.sol:1067-1068` writes the new deadline.
- **Test.** `tbtc-v2@b7aece4bf:solidity/test/bridge/Bridge.ReservationStrandingLibrary.test.ts:1670` "rejects when
  reanchor cooldown is in effect"; `tbtc-v2@b7aece4bf:solidity/test/bridge/Bridge.ReservationThirdPartyReanchorHolds.test.ts:1429` ("lets the reservation owner request past a running cooldown").
- **Status.** holds.

### INV-603 Term / dissolution-eligible / refund-deadline windows

The published code and the intended policy agree: a position on a healthy (Live)
wallet becomes strandable once `dissolutionEligibleAt` passes; the decision was
taken on 2026-10-29 and the code does not change.

- **Statement.**
  - `expiresAt = now + termSeconds` (`ReservationProofs.sol:557`).
  - `dissolutionEligibleAt = expiresAt + dissolutionDelay`
    (`ReservationProofs.sol:570`). A wallet's reservation may be stranded as
    follows: `Terminated` or `Closed` immediately; `Live` once
    `block.timestamp >= dissolutionEligibleAt`; `Closing` once **both**
    `block.timestamp >= dissolutionEligibleAt` **and**
    `block.timestamp >= closingStartedAt + walletClosingPeriod`
    (`tbtc-v2@b7aece4bf:solidity/contracts/bridge/Reservation.sol:1350-1361`).
    The closing-period half is an exclusive re-anchor window, not an extra
    delay on dissolution.
  - Late acceptance settlement is allowed iff
    `block.timestamp <= timeoutAt + termSeconds` (`tbtc-v2@b7aece4bf:solidity/contracts/bridge/ReservationProofs.sol:204-211`,
    equality allowed, `"Late acceptance settlement window expired"` at
    :210). Late re-anchor settlement carries no such bound.
  - Reveal caps the refund deadline: `refundDeadline <= now + term +
    DEPOSIT_REFUND_SAFETY_MARGIN` (`Deposit.sol:244-251`); a stale
    notification requires `block.timestamp > refundDeadline` (strict,
    equality NOT allowed) (`Reservation.sol:1248-1252`).
- **Why.** These equality/strict choices define the exact settlement windows;
  the stale `>` (strict) means the deadline instant itself is not yet stale.
- **Client check on a Closing wallet.** The keep-core stranding watcher gates a
  `Closing` wallet on both conditions. The dispatch sets the
  dissolution-eligible requirement for `Live` and `Closing`
  (`keep-core@96530a787:pkg/maintainer/spv/reservation_stranding_watch.go:232-234`),
  the loop compares the chain head with `DissolutionEligibleAt` (`:356-368`), and
  a `Closing` wallet additionally has to be past
  `closingStartedAt + walletClosingPeriod` (`:379-392`), so the client no longer
  sends the notification the contract would revert
  (`tbtc-v2@b7aece4bf:solidity/contracts/bridge/Reservation.sol:1349-1361`).
  A wallet that fails either gate sets the `recheck` flag and is retried on a
  later pass. Before keep-core #4365 the `Closing` check read the
  dissolution-eligible time alone and logged the revert without setting the
  flag; the `Live` branch did not exist. It strands nothing early and moves no
  funds.
- **Test.** `Reservation.test.ts` "should succeed when timeoutAt + 24h ==
  refundDeadline (close bound exact)" (:405) and "should revert when
  timeoutAt + 24h == refundDeadline + 1s" (:433);
  `Bridge.ReservationStrandingLibrary.test.ts` "succeeds when reservation is
  dissolution-eligible" (:1366); at `tbtc-v2@b7aece4bf`:
  `tbtc-v2@b7aece4bf:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:222`
  (Closing rejected past dissolution eligibility while the closing period runs),
  `:263` and `:313` (stranding once both conditions hold, inclusive at the end
  of the closing period), `:386` and `:397` (a `Live` wallet before and after
  dissolution eligibility), and
  `tbtc-v2@b7aece4bf:solidity/test/bridge/ReservationProofs.test.ts:446`
  (late acceptance one second past the window reverts), `:456`
  (`"Late acceptance settlement window expired"`).
- **Status.** holds at `tbtc-v2@b7aece4bf` for the corrected windows. The
  `Closing` clause recorded before VF-10 named dissolution eligibility only and
  did not match the published code (VF-10).

---

## 7. Cross-contract

### INV-701 The vault mints gross and retains the fee; debt is the only mechanism that can under-cover a miner fee

- **Statement.** On a credit, `receiveBalanceIncrease` mints
  `sats * SATOSHI_MULTIPLIER` of TBTC and transfers each depositor
  `gross - fee`, keeping `fee = gross * initiationFeeBps / 10000` in the
  vault as the in-kind reserve. A miner fee that the reserve cannot cover is
  the only increment of `inKindFeeDebtSat` within this vault.
  Other protocol minting and loss paths are outside this local invariant.
- **Enforcing code.** `ReservationVault.sol:161-195` (mint + fee distribution
  at :178-192), `:215-236` (`financeInKindFee`, debt at :229).
- **Why.** Defines the callback's distribution and local fee-debt writer.
  It does not prove reservation-key provenance or total pooled solvency.
- **Test.** `ReservationVault.test.ts` "should transfer the net amount (gross
  minus fee) to the depositor" (:201), "should retain the fee in the vault"
  (:205), "should leave no residual Bank balance for the vault or depositor"
  (:209), "should record the entire fee as debt" (:379).
- **Status.** holds.

### INV-702 Acceptance settlement uses the deposit's stored vault, subject to current trust

- **Statement.** Settlement reads `deposit.vault`, not the current configured
  `reservationVault`. D26 can change the stored vault only before any acceptance request.
  After a request, the release guard preserves that routing field.
  If its trust is revoked, settlement directly credits the depositor's Bank balance.
- **Enforcing code.** `ReservationProofs.sol:599-631` (the
  `isVaultTrusted` gate at :621-624).
- **Why.** Governance can re-point or disable the live vault between the
  authorization and the proof; the deposit's original vault must win.
- **Test.** `ReservationProofs.test.ts` "should credit the deposit's
  immutable vault, not a reservationVault repointed mid-flight" (:1757).
- **Status.** holds.

### INV-703 `setVaultStatus(vault, true)` is only safe at/after the keep-core activation block B

- **Statement.** Public-network activation requires a client build containing B
  and operator readiness before governance trusts the reservation vault at or after B.
  An entry returns B regardless of the current block.
  Client callers compare their work block with B.
  A missing network entry returns `math.MaxUint64`, not merely a pre-B entry.
- **Enforcing code.** `keep-core:pkg/tbtc/coordination.go:94-120` defines
  the network table and fallback.
  `setVaultStatus` has no on-chain B guard (`Bridge.sol:1288-1294`).
  This is a deployment requirement, not an unconditional contract invariant.
- **Why.** Prevents activation before clients can coordinate signed anchors.
  It does not replace vault-credit provenance validation or guarantee SPV availability.
- **Test.** `TestReservationsActivationBlock_SanityCheck` (`keep-core@96530a787:pkg/tbtc/coordination_test.go:1667`) pins that `Mainnet` and `Unknown` have no entry and therefore never activate; lab scenario H-1 `resv-activation-order` PASS (7,191 s, run `20261029t204323z-piotrroslani-b12c`) gates the deploy-order half, reading the vault-trust event block against B and the head before it.
- **Status.** `holds` for the client lookup behavior and the deploy-order gate as the lab scenario exercises it. The residual is that the activation block B is still unchosen, so the ceremony gate ran with a lab-chosen B.
- **Outstanding prerequisite (owner undecided, due before activation).** The
  activation block B is not yet chosen, `feeReserveTarget` is not yet seeded, and the
  irreversibility sign-off has not been given. This set does not name who owns them;
  the decision is still open (see DR-07).

---

### INV-704 Units, gross mint, net transfer and fee reserve

- **Statement.** One satoshi is `10^10` TBTC base units.
  For an anchored credit a: `grossWei = a * 10^10`,
  `feeWei = floor(grossWei * initiationFeeBps / 10000)` and
  `netWei = grossWei - feeWei`.
  Fee distribution does not itself destroy BTC backing or burn supply.
  The fee remains in the vault unless a later explicit operation moves or burns it.
- **Enforcing code.** `tbtc-v2:solidity/contracts/vault/ReservationVault.sol:175-196`
  computes the gross mint and depositor transfer.
- **Test.** `keep-core@680300533:pkg/tbtcpg/reservation_satoshi_units_test.go:131` ("TestSatoshiToTbtcBaseUnits_Vector") and `keep-core@680300533:pkg/tbtcpg/reservation_satoshi_units_test.go:170` ("TestSatoshiToTbtcBaseUnits_Constant") and `keep-core@680300533:pkg/tbtcpg/reservation_satoshi_units_test.go:334` ("TestSatoshiToTbtcBaseUnits_VaultFeeReserveScale"); and `tbtc-v2@b7aece4bf:solidity/test-foundry/reservations/ReservationSatoshiUnits.t.sol:94` ("test_vectorFileStatesTheAgreedFactor") and `tbtc-v2@b7aece4bf:solidity/test-foundry/reservations/ReservationSatoshiUnits.t.sol:118` ("test_vectorFileContainsTheDefiningPair") and `tbtc-v2@b7aece4bf:solidity/test-foundry/reservations/ReservationSatoshiUnits.t.sol:138` ("test_vaultSatoshiMultiplierIsTenToTheTen") and `tbtc-v2@b7aece4bf:solidity/test-foundry/reservations/ReservationSatoshiUnits.t.sol:159` ("test_vaultMultiplierMatchesEverySharedVectorPair") and `tbtc-v2@b7aece4bf:solidity/test-foundry/reservations/ReservationSatoshiUnits.t.sol:192` ("test_vaultFeeArithmeticOnASharedVectorPair") and `tbtc-v2@b7aece4bf:solidity/test-foundry/reservations/ReservationSatoshiUnits.t.sol:266` ("test_tbtcVaultConstantAndConversionUseTheSameFactor") and `tbtc-v2@b7aece4bf:solidity/test-foundry/reservations/ReservationSatoshiUnits.t.sol:321` ("test_conversionTruncatesTheRemainderLikeTheSdk") tie the satoshi to the TBTC base unit and to the vault's fee-reserve scale; the contract half is the vault's `SATOSHI_MULTIPLIER` at `tbtc-v2@b7aece4bf:solidity/contracts/vault/ReservationVault.sol:84` and the net-transfer and fee-retention cases cited in INV-701. The branch head is `680300533` on keep-core `spec/c-rows-go`, an ancestor of the held keep-core head `prep/kc-freeze-candidates` at `8e7e2b30f`; the freeze-day keep-core push publishes both.
- **Status.** `holds` for the combined cross-repo unit property, client side and contract side: the three client cases fail on every pair when the factor is changed from 1e10 to 1e9, and the seven Foundry cases fail when the vault's `SATOSHI_MULTIPLIER` stops being 10^10, when the shared vector stops matching the vault, when the vault fee arithmetic stops matching a shared vector pair, or when the contract conversion stops truncating the remainder the way the SDK does. The shared vector file is the single source both sides read, which is what makes the cross-repo property checkable rather than asserted.

### INV-705 Miner-fee shortfall is an incremental backing difference, not the only possible system loss

- **Statement.** For a re-anchor fee f sats, let c be reserve coverage in whole sats.
  With unrelated activity held constant:
  `delta Bitcoin backing = -f`,
  `delta TBTC supply in sats = -c`, and
  `delta fee debt = f - c`.
  Therefore `delta(supplyInSats - backing) = delta feeDebt`.
  A later debt repayment burns supply without another Bitcoin miner fee.
  This does not assert that fee debt captures every possible pooled loss or uncredited spend.
- **Enforcing code.** `financeInKindFee` records the shortfall
  (`tbtc-v2@b7aece4bf:solidity/contracts/vault/ReservationVault.sol:231-249`).
  Repayment and debt-first sweeping decrease it (tbtc-v2@b7aece4bf:solidity/contracts/vault/ReservationVault.sol:257-283, tbtc-v2@b7aece4bf:solidity/contracts/vault/ReservationVault.sol:318-339).
  `_burnFromReserve` floors fractional-satoshi token balance before coverage
  (tbtc-v2@b7aece4bf:solidity/contracts/vault/ReservationVault.sol:392-407). A fractional satoshi does not cover a whole-satoshi fee.
- **Test.** `tbtc-v2@b7aece4bf:solidity/test/vault/ReservationVault.test.ts` shortfall, repayment and sweep cases cited in INV-207; the Medusa surface campaign (23 of 23 properties pass over 1,000,600 calls at `c8350d109`) executes `property_inKindFeeDebtMatchesFinancing`. `none` identified for the complete Bitcoin-plus-token conservation property.
- **Status.** `holds` for the incremental monetary property as the campaign executes it. The residual is that the conservation property is a vault-stack statement, not a Bitcoin-plus-token one, so no end-to-end Bitcoin-backed equality exists.

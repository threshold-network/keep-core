# Consumer SDK stories

Evidence pins and vocabulary: [user stories index](04-user-stories.md).

### D27 SDK reservation module (implemented at `tbtc-v2@7aabfc502`)

The D27 SDK module is now implemented (SDK commits `3d114b15e` and
`7768a3370`), not design-only. All SDK stories cite
`tbtc-v2@7aabfc502:typescript/<path>:<line>`; the service is the
`ReservationsService` (SM-C12), the handle is `EthereumReservations`
(SM-C11), the view surface is SM-C13, and the TOCTOU limit is SM-C14. SDK
tests live in `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts`
and use the `MockReservations`/`MockTBTCContracts` harness
(`typescript/test/utils/mock-reservations.ts`). A `go` or a preflight `ok`
never guarantees the on-chain call is accepted (SM-C14, US-98); it is an
off-chain prediction against live state.

D31 (hand-off reserve gate, at `tbtc-v2@806b1106f`, includes the SDK floor `dc5c2ed71`): the SDK reads the reserve through `EthereumReservations.reservationHandoffReserveWallets` (`tbtc-v2@806b1106f:typescript/src/lib/ethereum/reservations.ts:333`) and adds the gate to the pre-funding check (US-93) and the acceptance preflight (US-97). The story text below describes `7aabfc502`; each D31 update is a separate bullet with explicit `806b1106f` citations. Nothing is deployed on Sepolia or mainnet.

#### US-93 - Pre-funding go/no-go for a reserved deposit
- **Actor:** Depositor (SDK consumer).
- **Given/When/Then:** The consumer calls `checkReserveFeasibility({ amount,
  walletPublicKeyHash?, now? })` before funding. **Then** the service reads
  live `reservationParameters`, `reservationCaps`, `depositParameters` and
  `activeReservationsCount`, and returns `{ go, reasons[], refundLocktime?,
  revealBy?, window? }` where `reasons` are machine-readable no-go codes
  (`reservations-disabled`, `locktime-window-infeasible`,
  `locktime-slack-insufficient`, `amount-below-minimum`, `amount-below-dust`,
  `amount-above-single-cap`, `global-amount-cap-exceeded`,
  `global-count-cap-exceeded`, `wallet-not-live`, `wallet-count-cap-exceeded`,
  `wallet-amount-cap-exceeded`, `no-free-wallet`).
- **Functions:** `ReservationsService.checkReserveFeasibility`
  (`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:167`); vault-disabled short-circuit `:178`; window `:182`; amount/dust/cap reasons `:212`-`:229`; global-count `:233`-`:237` (checked even when the caller picks the wallet); wallet-cap reasons `:239`-`:256`; no-free-wallet `:257`-`:261`.
- **Test:** `reservations.test.ts` `checkReserveFeasibility` group - "should report go when all rules pass" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:108`, each no-go reason asserted in the "amount-below-minimum" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:202`, "amount-below-dust" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:219`, "amount-above-single-cap" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:237`, "global-amount-cap-exceeded" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:247`, "wallet-not-live" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:269`, "wallet-count-cap-exceeded" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:281`, "wallet-amount-cap-exceeded" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:297`, "global-count-cap-exceeded" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:313`/`tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:327`, "no-free-wallet" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:340` contexts. Status `holds`.
- **Status:** `holds` (SDK go/no-go logic unit-tested); the on-chain call it predicts is the acceptance request (US-03/US-04/US-05), re-checked on chain - the SDK result is not itself an acceptance guarantee (US-98).
- **D31 update: hand-off reserve gate in the pre-funding check (at `tbtc-v2@806b1106f`).** `checkReserveFeasibility` adds the no-go reason `handoff-reserve-exceeded` next to `global-count-cap-exceeded` (`tbtc-v2@806b1106f:typescript/src/services/reservations/reservations-service.ts:255-262`). It is evaluated from the same occupancy read and applies even when the caller picks the wallet. It mirrors the on-chain gate `activeReservationsCount + 1 <= max(0, liveWalletsCount - handoffReserveWallets) * maxReservationsPerWallet` (`tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1071-1077`, US-05). The reason list above is the `7aabfc502` list. Tests at `806b1106f` (describe "hand-off reserve gate" `tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:414`): both pre-funding paths refused when the gate is binding (`tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:434`), admitted at the boundary count = 1 (`tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:451`), refused while live wallets are at or below the reserve (`tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:463`), refused when the gate bites before the per-wallet caps (`tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:479`). Status `holds` at `806b1106f`. The SDK result is still not an acceptance guarantee (US-98).

#### US-94 - Refund locktime window and reveal-by deadline
- **Actor:** Depositor (SDK consumer).
- **Given/When/Then:** The SDK computes the on-chain locktime window
  `now + revealAheadPeriod <= locktime <= now + term + 24h`
  (`refundLocktimeWindow`, `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:708`); the chosen locktime is
  `min(lowerBound + revealSlack, upperBound - upperMargin)` with the default
  30-day reveal slack and 1-hour upper margin, and it must keep at least the
  1-day minimum reveal slack above the lower bound. **Then** `refundLocktime`
  and `revealBy` (locktime minus the reveal-ahead period) are reported; if the
  window is infeasible or the slack is insufficient the check is no-go.
- **Functions:** `ReservationsService.refundLocktimeWindow`
  (`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:708`); policy constants `RESERVATION_REVEAL_SLACK_SECONDS`
  `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:33`, `RESERVATION_UPPER_MARGIN_SECONDS` `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:40`,
  `RESERVATION_MIN_REVEAL_SLACK_SECONDS` `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:47`; `computeRefundLocktime`
  `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:727` (clamps to the >= 500M Bitcoin timestamp interpretation).
- **Test:** `reservations.test.ts` "should choose the 45-day locktime with the default 30-day reveal slack" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:118`; "with mainnet-like parameters" group "should choose the 180-day locktime (150d reveal-ahead + 30d slack)" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:372` and "should report locktime-slack-insufficient when the slack is below one day" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:393`. Status `holds`.
- **Status:** `holds`.

#### US-95 - Deterministic wallet capacity selection
- **Actor:** Depositor (SDK consumer).
- **Given/When/Then:** When no wallet is chosen, `selectReservationWallet(amount)`
  enumerates candidates from `NewWalletRegistered` events, filters to `Live`
  wallets with headroom against the per-wallet count cap
  (`maxReservationsPerWallet`) and the per-wallet amount cap
  (`maxReservationsAmountPerWallet`) for the intended `amount`, and picks the
  active wallet when it is eligible, else the lowest public-key-hash. **Then**
  the deposit is designated for that wallet. Both paths run the same
  pre-funding admission checks: the auto-selected path runs them in
  `checkReserveFeasibility` around the selection, and the caller-chosen path
  runs them against the named wallet, including its count and amount headroom
  (`tbtc-v2@86ce2e79b:typescript/src/services/reservations/reservations-service.ts:232-289`).
  At the SDK pin `7aabfc502` only the dust threshold and the global count were
  checked for a caller-chosen wallet (added by `7768a3370`); the full
  admission check for both paths is at `tbtc-v2@4c98e6b82`.
- **Functions:** `ReservationsService.selectReservationWallet(amount: BigNumber)`
  (`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:283`); enumeration `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:663`
  (`getNewWalletRegisteredEvents`); per-wallet count/amount filters `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:298`-`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:309`;
  active-wallet preference and lowest-pkh fallback `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:317`-`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:328`; the
  chosen-wallet dust (`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:216`-`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:220`) and global-count (`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:233`-`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:237`) checks
  are the `7768a3370` additions.
- **Test:** `reservations.test.ts` `selectReservationWallet` group - "should return undefined when no registered wallet is eligible" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:410`, "should skip wallets whose count cap is full" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:421`, "should skip wallets whose amount cap is full" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:438`, "should prefer the active wallet when it is eligible" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:459`, "should pick the lowest public key hash when no active wallet is set" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:475`; the chosen-wallet dust/global-count coverage is in `checkReserveFeasibility` "amount-below-dust" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:219` and "global-count-cap-exceeded" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:313`. Status `holds`.
- **Status:** `holds`.

#### US-96 - Reserved deposit initiation and reveal
- **Actor:** Depositor (SDK consumer).
- **Given/When/Then:** `initiateReservedDeposit(bitcoinRecoveryAddress, amount,
  options?)` generates a deposit receipt whose refund locktime lies inside
  the on-chain reservation window against the intended `amount`, designates a
  wallet, and returns `{ deposit, receipt, walletPublicKeyHash }`. The
  `amount` argument is required: the SDK pin `7aabfc502` still had the
  two-argument `initiateReservedDeposit(bitcoinRecoveryAddress, options?)`,
  and the published code adds `amount: BigNumberish` as the second parameter
  (`tbtc-v2@86ce2e79b:typescript/src/services/reservations/reservations-service.ts:387-391`). It throws when the default
  depositor is unset, reservations are disabled, no wallet resolves, or the
  locktime window is infeasible; the recovery address must be P2PKH/P2WPKH.
  `revealReservedDeposit(deposit)` then detects the funding output and calls
  `Bridge.revealDeposit` routed to the live `reservationVault`.
- **Functions:** `ReservationsService.initiateReservedDeposit`
  (`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:343`); `revealReservedDeposit` `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:419`;
  `generateReceipt` `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:733`; `refundPublicKeyHashFromRecovery` `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:750` (P2PKH/P2WPKH
  check `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:758`-`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:763`).
- **Test:** `reservations.test.ts` `initiateReservedDeposit` group - "should generate a receipt with a refund locktime inside the window" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:523`, "should use the explicitly chosen wallet" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:547`, "should reject a recovery address that is not P2PKH or P2WPKH" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:563`, "should throw when there is no active wallet and no registered wallet" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:571`; the disabled `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:510` and default-depositor `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:493` throws. Status `holds`.
- **Status:** `holds` (the SDK receipt generation and reveal wiring); the on-chain reveal is US-01 and the reserved-routing guards are US-02/US-04.

#### US-97 - Reserved reveal and acceptance request via the SDK
- **Actor:** Depositor (SDK consumer, the connected signer).
- **Given/When/Then:** `requestAcceptance(depositKey, walletPublicKeyHash)`
  submits the on-chain `requestReservationAcceptance` (US-03) for the
  depositor. `checkAcceptancePreflight(depositKey, walletPublicKeyHash)`
  reports which on-chain request rule would fail without submitting: vault
  set, deposit revealed, not swept, caller-is-depositor, flagged reserved,
  no request already made, wallet is the designated wallet, wallet Live,
  amount >= min+fee, single-amount cap, global-count cap, per-wallet count
  and amount caps. The deadline-based signing-window rules are NOT checked
  (the snapshotted refund deadline is not a public view). The owner address is
  read as `reservationRecord.owner` (zero -> `undefined`).
- **Functions:** `ReservationsService.requestAcceptance`
  (`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:456`); `checkAcceptancePreflight` `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:478`;
  owner/depositor match `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:505`; designated-wallet `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:521`; Live `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:526`;
  amount `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:530`; caps `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:534`-`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:555`; owner read at
  `EthereumReservations.reservations` `tbtc-v2@7aabfc502:typescript/src/lib/ethereum/reservations.ts:236`.
- **Test:** `reservations.test.ts` `requestAcceptance` group - "should submit the acceptance request to the designated wallet" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:593`, "should preflight-clean for a valid deposit" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:604`, and each negative preflight: not-revealed `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:614`, already-swept `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:629`, not-reserved `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:644`, request-already-made `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:658`, wrong-designated-wallet `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:671`, wallet-not-live `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:682`, below-minimum `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:693`, single-cap `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:711`, global-count-cap `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:724`, per-wallet-count-cap `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:737`, per-wallet-amount-cap `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:751`, proxy-depositor `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:769`. Status `holds`.
- **Status:** `holds` (SDK preflight logic); the on-chain request is US-03/US-04/US-05 and is re-checked on chain (US-98).
- **Target update (VF-12, at `tbtc-v2@4c98e6b82`).** `requestAcceptance(fundingTxHash, fundingOutputIndex, walletPublicKeyHash)` reads the deposit's funding transaction, assembles the SPV proof at the difficulty `txProofDifficultyFactor()` requires, throws before sending when the transaction is too shallow ("Deposit not funded yet" covers the unfunded case in the reveal path), and submits the four-argument `requestReservationAcceptance(fundingTx, fundingProof, fundingOutputIndex, walletPublicKeyHash)` (`tbtc-v2@4c98e6b82:typescript/src/services/reservations/reservations-service.ts:539-563`, assembly at :546-556). The preflight rules are unchanged. The earlier `requestAcceptance(depositKey, walletPublicKeyHash)` signature and its citations stay as history; at the published target the SDK does not submit a key-only request, so the two-argument statement does not match the published code. Test: `tbtc-v2@86ce2e79b:typescript/test/services/reservations.test.ts:1017` ("should submit the funding transaction, its proof and the output index").
- **D31 update: hand-off reserve gate in the acceptance preflight (at `tbtc-v2@806b1106f`).** `checkAcceptancePreflight` also reports the on-chain reserve gate of `reserveAcceptanceCapacity` (US-05, row "Hand-off reserve", `tbtc-v2@806b1106f:solidity/contracts/bridge/Reservation.sol:1076`). When the count of open positions plus one would exceed `max(0, liveWalletsCount - handoffReserveWallets) * maxReservationsPerWallet`, the preflight adds the failure "the hand-off reserve gate would be exceeded" (`tbtc-v2@806b1106f:typescript/src/services/reservations/reservations-service.ts:567-574`; computation in `handoffReserveExceeded` `tbtc-v2@806b1106f:typescript/src/services/reservations/reservations-service.ts:707-722`). The two inputs are read from the live Bridge through `EthereumReservations.reservationHandoffReserveWallets` (`tbtc-v2@806b1106f:typescript/src/lib/ethereum/reservations.ts:333`) and `liveWalletsCount`. Tests at `806b1106f` (describe "hand-off reserve gate" `tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:414`): the preflight fails when the gate is binding while the ordinary caps have room (`tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:514`), passes at the boundary count = 1 (`tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:528`), fails while live wallets are at or below the reserve (`tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:542`); the service exposes the live wallet count and the reserve (`tbtc-v2@806b1106f:typescript/test/services/reservations.test.ts:559`). Status `holds` at `806b1106f` (SDK preflight logic); the on-chain gate is re-checked on chain (US-98). The caller-visible failure list above is the original `7aabfc502` list.

#### US-98 - SDK pre-funding/preflight does not guarantee acceptance (TOCTOU limit)
- **Actor:** Depositor (SDK consumer).
- **Given/When/Then:** A `go` from `checkReserveFeasibility` or an `ok` from
  `checkAcceptancePreflight` is an off-chain prediction read against live
  on-chain state. Between that read and the on-chain request, capacity,
  wallet Live state, dust, caps, the owner/depositor match, and the signing
  window can all change; the on-chain `requestReservationAcceptance`
  re-checks every rule and may still revert. The SDK also cannot evaluate the
  deadline-based signing-window rules (the snapshotted refund deadline is not
  a public view), so a preflight `ok` says nothing about whether the
  authorization window still admits a signature.
- **Functions:** `checkReserveFeasibility` and `checkAcceptancePreflight`
  read-then-write boundary (`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:167`, `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:478`); on-chain
  re-checks at `Reservation.sol:512` (depositor), `:554` (Live), `:562`
  (amount), `:586` (signing window); preflight doc "The deadline-based
  signing-window rules are not checked here" `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:470`.
- **Test:** the SDK checks themselves are unit-tested (US-93, US-97); the
  guarantee they do NOT provide is the on-chain re-check, pinned by the on-chain
  US-04/US-05 tests (`Reservation.test.ts` guards), not by any SDK assertion. Status `holds-untested` (SDK-to-on-chain TOCTOU boundary).
- **Status:** `holds-untested` (the boundary is a documented limit, SM-C14); the SDK never implies the preflight guarantees acceptance.
- **D31 note (at `tbtc-v2@806b1106f`):** the hand-off reserve gate adds one more input that can change between the SDK read and the on-chain request: `liveWalletsCount` falls when a Live wallet leaves Live, which tightens the budget (on-chain test `tbtc-v2@806b1106f:solidity/test/bridge/Bridge.ReservationOccupancy.test.ts:346`). The SDK check `handoffReserveExceeded` returns false when `maxReservationsPerWallet` is 0, while on-chain `N = 0` would reject every acceptance (`tbtc-v2@806b1106f:typescript/src/services/reservations/reservations-service.ts:707-722`); the SDK treats that as an unset cap, so a `go` or `ok` there is still only a prediction. Status unchanged: `holds-untested`.

#### US-99 - Release-to-pool and status reads via the SDK
- **Actor:** Depositor (SDK consumer).
- **Given/When/Then:** `releaseToPool(depositKey, vault?)` submits the
  on-chain `releaseReservedDepositToPool` (US-90/US-92), defaulting the
  vault to the zero address when omitted (`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:574`-`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:583`). The status reads
  `reservationRecord`, `reservationAction`, `reservationParameters` and
  `isReservedDeposit` return the on-chain position, action generation,
  parameters, and pending-reserved flag respectively.
- **Functions:** `ReservationsService.releaseToPool`
  (`tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:574`); status reads `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:590` (`reservationRecord`),
  `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:600` (`reservationAction`), `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:614` (`reservationParameters`), `tbtc-v2@7aabfc502:typescript/src/services/reservations/reservations-service.ts:623`
  (`isReservedDeposit`).
- **Test:** `reservations.test.ts` `releaseToPool` group - "should release to the zero address when no vault is given" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:793`, "should release to the given vault" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:803`; `status reads` group - "should read the reservation record" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:815`, "should read the reservation action record" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:822`, "should read the reservation parameters" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:829`, "should read whether a deposit is reserved" `tbtc-v2@7aabfc502:typescript/test/services/reservations.test.ts:836`. Status `holds`.
- **Status:** `holds` (SDK release and status-read wiring); the on-chain release is US-90/US-92 and the reads mirror the SM-C13 views.

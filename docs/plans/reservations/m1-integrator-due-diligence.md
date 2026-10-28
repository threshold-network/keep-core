# M1 UTXO reservations - integrator due diligence

Written 2026-10-12 for a third party with no prior knowledge of tBTC who wants to integrate reservations.
- Contracts read: tbtc-v2 the integrated reservation branch @ `9d208696`.
- Client read: keep-core the drill-fix branch @ `c102f4b85`.
- Companion: `m1-guide.md` explains how M1 works.

`[INFERENCE]` marks claims that are reasoned, not read in code.

## 0. Status in one line

M1 is implemented and reviewed internally. It is not merged to `dev`, not deployed on Sepolia or mainnet, and has no external audit. M1 is "rails, not product": a user can create a reservation but cannot redeem it in kind.

## 1. Questions an integrator will ask

### Product
- **What does my user get?** TBTC, minted 1:1 against a separate Bitcoin output (the anchor). The deductions:
  - the anchor transaction's miner fee, capped by the snapshotted `txMaxFee`;
  - 0.4% (`ReservationVault.initiationFeeBps = 40`, `ReservationVault.sol:135`).

  The normal tBTC treasury fee does not apply.
- **Can my user get their exact BTC back?** Not in M1. In-kind redemption is M2. It needs a new vault and a migration that each owner opts into. In M1 the user holds ordinary TBTC and can redeem through the pool or sell it.
- **What is the in-kind right worth over time?**
  - Each re-anchor reduces `anchorAmount` (the in-kind amount) by that hop's miner fee. The user's TBTC balance does not change.
  - The term is a promise clock. If M2 has not shipped by the position's `dissolutionEligibleAt` (planned term 12 months, plus a delay), the spec says that group's in-kind option lapses quietly.
- **Who owns the position? Can it be transferred?**
  - The owner is the depositor's Ethereum address (`ReservationProofs.sol:559`). M1 has no way to change the owner.
  - A contract owner must be able to call `requestReservationAcceptance` now, and probably the M2 migration later. The M2 interface is not designed yet.

### Mechanics
- **Bitcoin wallet requirements:** pay a normal tBTC deposit address (P2SH or P2WSH). If the deposit is never accepted, the wallet must also be able to sign a script-path refund after the locktime.
- **Ethereum requirements:**
  - Reveal with `vault = ReservationVault`, from the depositor address written in the script.
  - Call `requestReservationAcceptance` (depositor only: `Reservation.sol:511-513`). The depositor pays gas for both calls.
  - SPV maintainers pay for proofs; the maintainer proxy does not reimburse them.
- **Wallet choice:** the tBTC wallet is written into the deposit script and cannot change later. It must be `Live` and have free capacity. With `maxReservationsPerWallet = 1`, each wallet holds at most one reservation at a time.
- **Time to mint:** `[INFERENCE]` several hours. The deposit must be at least 2 h old, coordination windows come every 900 blocks (about 3 h), then the anchor needs Bitcoin confirmations and an SPV proof.
- **What to monitor:**

  | Kind | Names |
  |---|---|
  | Events | `ReservationAcceptanceRequested`, `ReservationAccepted`, `ReservationAcceptanceTimedOut`, `ReservationReanchorRequested`, `ReservationReanchored`, `ReservationStranded`, `ReservedDepositMarkedStale`, `ReservationLateSettled` |
  | Views | `reservations(key)`, `reservationActions(key, nonce)`, `reservedDepositWallet(key)`, `pendingReservedDeposits()` |

### Trust and governance
- **Who can change the parameters?** BridgeGovernance, with a delay (48 h on mainnet). A change never affects a pending action, because its rules were frozen when it was requested.
- **Who can change the code?**
  - The Bridge and the reservation router: a proxy upgrade by the proxy admin.
  - The vault cannot be upgraded. Replacing it needs zero open positions and zero pending reserved deposits.

## 2. Edge cases

| # | Case | What happens | What to do |
|---|---|---|---|
| E1 | Locktime outside the window | The reveal reverts ("Refund locktime too far in the future for a reservation" or "too close"). The BTC is already sent and stays locked until the refund locktime. | Before funding, check `now + revealAhead <= locktime <= now + term + 24h`, with a margin for reveal delay. |
| E2 | Long lockup by design | On mainnet's current values (reveal-ahead 150 days), a reserved deposit's locktime is at least about 150 days away. A deposit that is never accepted waits that long. | Tell users this before they deposit. |
| E3 | No capacity at request time | `requestReservationAcceptance` reverts: "Active reservations cap exceeded", "Wallet reservations cap exceeded", or an amount cap. The reserved mark is permanent: such a deposit can never be swept into the pool (`WalletProposalValidator.sol:313`), and marking it stale does not clear the mark. The only ways out are freed capacity, or stale plus a Bitcoin refund. | Read the capacity views just before funding. Request at once after reveal. Expect races while caps are small. |
| E4 | Amount out of bounds | Below `minAmount + txMaxFee`, or above the single cap or the per-wallet amount cap: the request reverts. The deposit is still reserved, so E3 applies. | Check the amount before funding. |
| E5 | Request too late | The request needs `timeoutAt + 24h <= refundDeadline` and a 2 h signing window, or it reverts ("would overlap the deposit refund window"). | Request soon after the 2 h minimum age. |
| E6 | Designated wallet leaves `Live` | Before the request: the request reverts. Between the request and the proof: the position is stranded as soon as the proof lands. | Choose a healthy wallet. Watch for `ReservationStranded`. |
| E7 | Wallet does not sign in time | Anyone times out the acceptance, capacity is released, and the depositor may request again. No slashing. | Retry the request. |
| E8 | Fee bumps | A replaced funding transaction gets a new ID, so reveal only the confirmed one. The anchor transaction can also be replaced; the maintainer proves whichever version confirms first. | Reveal after confirmation. |
| E9 | Lost Ethereum key before the request | Nobody else can request acceptance. The deposit goes stale and the user refunds on Bitcoin. | Treat the Ethereum key as critical until acceptance. |
| E10 | Lost Ethereum key after acceptance | The TBTC is lost with the key. The in-kind right and the M2 migration are tied to that address. | Use a recoverable owner (multisig or contract). |
| E11 | Lost Bitcoin refund key | Harmless if the deposit is accepted. If it is never accepted, the BTC is stuck forever. | Back up the refund key, or use a wallet that can sign scripts. |
| E12 | Re-anchor fees | Each hop reduces the in-kind amount. The only floor is `anchor > txMaxFee + minAmount`; there is no cap on total fees across hops. | Show `anchorAmount` to users. |
| E13 | Griefing re-anchors | Third parties remain subject to the hold cap and ordinary target caps. Privileged hand-offs from retiring sources may exceed ordinary target caps, so a hold on another position cannot block evacuation merely by filling a target. A pending action on the same position and its timeout cooldown remain relevant. | Monitor pending actions and deadlines. |
| E14 | Proof outage longer than the late window | See D3 below. A confirmed anchor can end up with no TBTC credited. | Monitor proof liveness; escalate early. |

## 3. Disaster recovery

The guarantees are narrower than "everything settles". Three facts frame this section:
- Caps and fee-reserve shortfalls never block a proof.
- A late acceptance proof is accepted only until `timeoutAt + termSeconds` (`ReservationProofs.sol:204-211`). A late re-anchor proof has no time limit while the source anchor is current.
- A user's **nominal** TBTC balance is never reduced by a reservation failure. Its **backing** can be: a stranded anchor is a shortfall shared by every TBTC holder, including the owner.

| # | Scenario | Automatic recovery | Fallback | Outcome |
|---|---|---|---|---|
| D1 | keep-core operators offline | None. Timeouts are permissionless, and keep-core watchers also call them | Timeouts release capacity. An unaccepted deposit goes stale after its refund deadline | Delay. If no anchor was signed, the depositor refunds on Bitcoin after the locktime |
| D2 | SPV maintainers offline, short outage | None | Any allow-listed maintainer submits later. Late proofs are accepted within the window | Delay only |
| D3 | SPV maintainers offline past `timeoutAt + termSeconds` after a signed, confirmed anchor | None | Escalate to governance. `[INFERENCE]` No on-chain path in M1 credits it any more: the deposit's refund output is already spent, and the anchor is not tracked. Recovery needs a governance decision, most likely a Bridge upgrade | Real risk of an uncredited deposit. The window is long (about 12 months plus the action timeout on planned mainnet values), so this needs a sustained outage. Monitor proof submission and acceptance timeouts, and escalate within days, not months |
| D4 | Wallet retires normally (`MovingFunds`) | The keep-core re-anchor task moves each anchor to a `Live` wallet, then files the below-dust report so the wallet can close | Anyone can request a re-anchor (hold cap applies). Governance can rotate | Position continues; the in-kind amount drops by one miner fee |
| D5 | Wallet `Closing` past `dissolutionEligibleAt`, or `Closed`, with a position on it | Stranding watcher | Anyone can call `notifyReservationStranded` (`Reservation.sol:1207-1216`) | Normal-close cleanup. Nominal TBTC kept; in-kind right lost |
| D6 | Wallet dies (fraud, or a moving-funds timeout) | Stranding watcher | Anyone can call it. If a re-anchor was already signed and timed out, a late proof of it restores the position | Nominal TBTC kept; the shortfall is shared by all TBTC holders. No compensation exists in M1 (no Tier 0/1 code; `ReservationStranded` is only evidence) |
| D7 | Saturation of ordinary target capacity | keep-core uses a full eligible Live target for a source-member hand-off when no target has headroom | Governance retains delayed control of normal count and amount limits. The admission reserve stops new acceptances beyond the current budget | Existing privileged evacuation is not blocked by ordinary caps. No surviving Live target, missing quorum and proof outages remain liveness risks |
| D8 | Vault fee reserve empty | The re-anchor settles; the shortfall becomes `inKindFeeDebtSat` | Anyone can repay; `sweepFees` repays debt first | No direct user impact; the `reservation_vault_fee_debt_sat` alarm fires |
| D9 | Bad parameters (no deposit possible) | The Bridge guard refuses such values | `reservation_params_infeasible` alarm | Reveals blocked before anyone funds |
| D10 | Emergency stop of new reservations | `setVaultStatus(vault, false)` stops reserved reveals at once | Pending items still settle. Late acceptances credit the depositor directly, without the 0.4% fee | Existing users unaffected |
| D11 | Bug in reservation logic | None | A Bridge proxy upgrade replaces the router code. The vault cannot be upgraded | Depends on the bug. A vault bug needs a new vault plus migration |
| D12 | Bitcoin reorg | Proofs need the Bridge's confirmation depth; the maintainer scan stays 12 Ethereum blocks behind | Same as ordinary tBTC | Same as ordinary tBTC |

## 4. Red flags for a careful outsider

1. **No exit for the owner in M1, and no compensation for stranding.** Users keep nominal TBTC, but a stranding reduces backing for every holder, the owner included.
2. **Mistakes before funding are expensive** (E1-E5). A reserved deposit can never fall back into the pool. Its only way out is a Bitcoin refund after a locktime at least about 150 days away on mainnet.
3. **The acceptance late-proof window is bounded** (D3). A long proof outage can leave a confirmed anchor uncredited.
4. **Capacity is tiny at launch.** With one reservation per wallet, the number of live wallets bounds the total.
5. **The M2 migration is a future duty for each owner.** Owner addresses must stay usable.
6. **No external audit yet, and nothing deployed.**

## 5. Recommendations

- **R1:** add a pre-funding check to the SDK. It returns go or no-go before the user sends BTC, covering:
  - locktime window;
  - amount bounds;
  - wallet `Live` and free capacity;
  - time left to request acceptance.

  It addresses E1-E5. Tracked as an M7 item in the epic tracker (approved 2026-10-12).
- **R2:** add an alert for acceptance generations that are close to `timeoutAt + termSeconds` and still unproven, with a governance escalation runbook (D3). Tracked as an M5 item (built and exercised in the drill; approved 2026-10-12).
- **R3:** state clearly in partner material: no in-kind exit in M1; the M2 migration needs the owner address; the lockup length for unaccepted deposits.

## 6. Wallet compatibility checklist

Use this to assess a depositor's Bitcoin wallet before an integration. Each row is a capability reservations need. The notes come from a review (2026-10-12) of one hardware-backed mobile wallet's source code; the vendor is not named here.

| Need | Why | What the reviewed wallet did |
|---|---|---|
| Pay a P2SH or P2WSH address | Funding the deposit | Yes |
| Give a 20-byte refund key hash (P2WPKH or P2PKH address) | The deposit script commits to it | Yes |
| Reveal and request acceptance from the same Ethereum address as the depositor | Both calls check `msg.sender` | Yes, through WalletConnect on Ethereum |
| Spend the deposit script after the locktime (refund) | The only exit for a deposit that is never accepted | **No.** It only spends its own single-key coins |
| Sign a PSBT input with a custom witness script, without finalizing it as single-key | A dApp could then build the refund | **No.** dApp PSBT signing was switched off in release builds. Even when on, it finalizes every witness input as `[signature, pubkey]`, which is invalid for a script spend. So switching it on would not make refunds work |
| Bitcoin testnet4 | tBTC on Sepolia uses testnet4 (keep-core `config/network/network.go:43`) | **No.** testnet3 only |
| Replace-by-fee | The transaction ID can change before confirmation | Always on, so reveal only after confirmation |

What a wallet that fails the refund rows means:
- **Deposits still work:** the user pays from the wallet, then sends the reveal and the acceptance request from its Ethereum account.
- **A deposit that is never accepted needs another way to refund:**
  - If the wallet was made with a seed phrase, import it into a wallet that can sign scripts (for example Sparrow or Electrum).
  - Or set the refund key to a separate wallet that can sign scripts.
  - Or the vendor adds script-aware PSBT finalization, proven by a real refund test.
- **A wallet without testnet4 cannot take part in the Sepolia drill.**
- **Questions for the vendor:** when Bitcoin dApp signing ships; whether it will support custom-script inputs with timelocks; whether all platforms behave the same; whether testnet4 is planned.

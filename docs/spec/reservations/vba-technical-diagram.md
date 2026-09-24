# Technical Diagram

{% hint style="warning" %}
This page describes v1 of the Verifiable Bitcoin Account, also called Milestone 1 ("M1"). A second
version (v2) is planned to ship later. The following features are planned and are NOT available in
M1:

- In-kind redemption: return the reserved bitcoin itself, for the whole position.
- Partial in-kind redemption: redeem part of a position; the rest stays reserved.
- Watchtower review for reserved redemptions, as for standard tBTC redemptions.
- Term renewal: extend a position's custody term.
- Dissolution: after the term and a delay, release a position so its wallet is no longer tied
  to it; the owner keeps their tBTC as an ordinary claim.
- A lifetime limit on how much re-anchor fees can shrink one position.

M1 positions carry over to v2 with the dates recorded at acceptance.
{% endhint %}

In M1, a Verifiable Bitcoin Account (VBA) is a UTXO reservation in the tBTC v2 Bridge. When bitcoin is deposited to a tBTC threshold wallet and revealed to the Bridge with the reservation vault named as its vault, it is never combined with other deposits: it keeps its own UTXO (the "anchor"), held by the same wallet that custodies pooled tBTC. tBTC is minted against that anchor, one position at a time. The engineering name is a "reservation"; this page uses both.

Every move of an anchor is a Bitcoin transaction with exactly one input and one output, proven to Ethereum with the Bridge's standard SPV checks. Custody security is the same as tBTC's: an honest majority of the threshold signers, with fraud challenges and stake slashing. The sections below show where each rule lives, how a position moves through its lifecycle, and how to verify any of it yourself.

***

## Pooled tBTC and a Verifiable Bitcoin Account

```
  POOLED DEPOSIT                          RESERVED DEPOSIT
  +----------------+                     +----------------+
  | deposits 1..n  |                     | deposit        |
  +-------+--------+                     +-------+--------+
          |  sweep                               |  reveal
          v                                      v
  +----------------+                     +----------------+
  | shared main    |                     | reserved       |
  | UTXO (pooled)  |                     | deposit        |
  +----------------+                     +-------+--------+
                                                 |  acceptance
                                                 |  proves anchor
                                                 v
                                         +----------------+
                                         | anchor UTXO    |
                                         | (own output,   |
                                         |  P2WPKH wallet)|
                                         +----------------+
  both sit under the same tBTC threshold wallet key;
  a reserved deposit is excluded from every sweep
```

What stays the same as ordinary tBTC:

- The deposit and reveal flow is identical; a reserved deposit just names the reservation vault at reveal.
- Custody is the same threshold wallet and signing quorum as pooled tBTC (51 of 100 seats on mainnet).
- The tBTC you receive is ordinary tBTC: freely transferable.
- Fraud protection is identical: proven spends are honest, others stay challengeable, and a failed challenge terminates the wallet and slashes operators.

What is different:

- The bitcoin is never swept into the wallet's main UTXO; the Bridge and signers reject any sweep containing a reserved deposit. The flag is permanent.
- Your position has its own traceable lineage: deposit UTXO, then anchor, then each re-anchor, each a one-input/one-output transaction.
- tBTC is minted against your anchor by the vault, which keeps an initiation fee as the reserve that funds future re-anchor miner fees.

***

## Components and roles

```
  PEOPLE
  +---------------------+   +----------------------------------------------+
  | Owner (depositor)   |   | Threshold Council                            |
  +---------------------+   +----------------------------------------------+
      |                        |                            |
      | reveal deposit,        | parameters, caps           | fee settings
      | request acceptance     | (governance delay)         | (instant)
      v                        v                            v
  ETHEREUM
  +--------------------------------------+      +--------------------------+
  | tBTC v2 Bridge with reservation code |      | ReservationVault         |
  | records positions, authorizes        |credit| mints tBTC and pays the  |
  | every move, checks caps and          |----->| owner, minus the         |
  | Bitcoin proofs, keeps the claim      |      | initiation fee; its fee  |
  | equal to the anchor                  |      | reserve pays re-anchor   |
  |                                      |      | miner fees               |
  +--------------------------------------+      +--------------------------+
      ^                ^              ^
      | check          | prove        | notify: timeouts,
      | authorization  | Bitcoin txs  | stale deposits, stranding
      |                |              |
  OFF-CHAIN (signer operators)
  +-------------+ +--------------+ +------------+
  | tBTC signer | | SPV          | | watchers   |
  | nodes       | | maintainers  | |            |
  +-------------+ +--------------+ +------------+
      |
      | sign anchor and re-anchor transactions
      v
  BITCOIN
  deposit UTXO --> anchor UTXO --> next anchor UTXO (1 in, 1 out each)
```

| Role | What it does | What it cannot do |
|------|--------------|-------------------|
| Owner (depositor) | Sends and reveals the deposit; requests acceptance; receives the tBTC | Move, redeem or close the position in M1; spend the anchor |
| tBTC threshold wallet and its signer nodes | Custodies the anchor; proposes and signs anchor and re-anchor transactions | Include a reserved deposit in any sweep; sign a transaction outside a pending Bridge authorization (a fraud challenge punishes it) |
| SPV maintainers | Prove confirmed anchor and re-anchor transactions to the Bridge | Forge a proof or change what a proof settles; only authorized maintainers can submit proofs |
| Watchers (signer software) | File the permissionless housekeeping calls: action timeouts, stale deposits, stranding | Grant, block or speed anything up; all such calls are open to anyone |
| Threshold Council (Bridge governance and vault owner) | Sets parameters and caps with a governance delay; changes the vault's initiation fee (hard-capped at 5%) and fee reserve target instantly; requests re-anchors off healthy Live wallets | Change values in ways the Bridge rejects; move a position without a signed, proven transaction |
| Anyone | Repay the vault's public fee debt; request re-anchors from retiring wallets; file housekeeping notifications | Request acceptance for someone else's deposit; re-anchor a position off a Live wallet (governance only); send a position anywhere except another Live tBTC wallet |

***

## Where each rule is enforced

| Rule | Where | How |
|------|-------|-----|
| Only the wallet's key can spend an anchor | Bitcoin | The anchor is a P2WPKH (native SegWit) output of the tBTC wallet; Bitcoin checks the wallet's signature and nothing else. A reserved deposit's refund locktime also protects the pre-acceptance path: an unaccepted deposit always becomes refundable on Bitcoin. |
| Every move is authorized in advance | Bridge (Ethereum) | Each move starts as a pending authorization (Acceptance or Reanchor) with a deadline and snapshots of the parameters that matter. No transaction settles without its authorization. |
| Caps hold | Bridge | Single-reservation, per-wallet count, per-wallet amount, global count and global amount caps are all checked when an action is requested, and capacity is reserved until settlement. |
| Exact transaction shape | Bridge | At proof time the Bridge verifies the SPV proof and checks the Bitcoin transaction: exactly one input, exactly one output paying the wallet's key, fee within the snapshotted maximum, amount within the rules. |
| Claim equals anchor | Bridge | The Bridge writes the minted claim and the anchor amount together, at acceptance and at every re-anchor; the two are never changed separately. |
| Signers only sign authorized moves | Signer nodes (off-chain) | Every signer re-checks the on-chain authorization before signing a reservation transaction; unauthorized proposals are hard-rejected. |
| Unauthorized signatures are punished | Ethereum (fraud challenges) | Proven anchor and re-anchor spends are recorded as honest; any other signature over an anchor remains challengeable, and a wallet that cannot defeat a challenge is terminated and its operators' stake slashed. |

The rules do not live in a per-account Bitcoin script. On the bitcoin side, one fact holds: only the tBTC wallet's key can spend the anchor. Everything else - which moves are allowed, to which wallet, how large, when, under which caps - is enforced by the Bridge's reservations extension on Ethereum. The bitcoin transaction is proven to Ethereum; Ethereum decides whether it counts.

***

## Lifecycle

```
   +-----------+
   | activation|  governance switches M1 on: vault trusted, caps set
   +-----+-----+
         |
         v
   +----------------+
   | deposit and    |  bitcoin sent to the tBTC wallet, revealed
   | reveal         |  to the reservation vault
   +-------+--------+
           |
           v
   +----------------+
   | acceptance     |  depositor requests on Ethereum; Bridge
   | request        |  validates and reserves capacity
   +-------+--------+
           |
           v
   +----------------+
   | anchoring on   |  one Bitcoin transaction moves the deposit
   | Bitcoin        |  into its own anchor UTXO
   +-------+--------+
           |
           v
   +----------------+
   | proof and      |  Bridge writes the position and credits the
   | minting        |  vault; the vault mints tBTC, pays the owner
   +-------+--------+
           |
           v
   +----------------+       +------------------+
   | custody        | --->  | re-anchor        |
   | (position      | <---  | (repeatable)     |
   |  open, Active) |       +------------------+
   +----+-----------+   when a wallet retires,
        |                  the anchor moves to a
        |                  different Live wallet
        v
   M1 end state: a position ends only if its custodying wallet dies
   (Stranded). Exits such as in-kind redemption, renewal and
   dissolution arrive in v2.
```

### 1. Activation

M1 is switched on by governance. The reservation machinery ships inert: the router is registered with the Bridge, the vault is deployed but untrusted, and deposits cannot be revealed to it. Governance sets the parameters and caps through the delayed governance process, then marks the vault trusted. That final switch is when reservations become possible. Signer software enables reservation duties only on networks where M1 is activated.

### 2. Deposit and reveal

Send bitcoin to a tBTC threshold wallet's deposit script, then reveal the deposit on Ethereum. A deposit becomes reserved by naming the reservation vault at reveal time; the wallet committed to in the deposit script is the designated wallet, and only it can accept the deposit. Reserved deposits must use a refund locktime no later than the reveal time plus the term plus 24 hours, so an unaccepted deposit always becomes refundable on Bitcoin. No reservation minimum or cap is checked at reveal: only the ordinary dust threshold.

{% hint style="warning" %}
Capacity is checked when acceptance is requested, not when the bitcoin is sent. Before sending, check that the designated wallet has a free slot under its caps and that the global caps have room. Otherwise the acceptance request will fail, and you will have to wait for the bitcoin to become refundable through the deposit script's refund path.
{% endhint %}

Reserved deposits are not charged the Bridge's normal deposit treasury fee; the vault's initiation fee applies instead (see Amounts and fees).

### 3. Acceptance: request, then proof

```
  1. DEPOSITOR, on Ethereum: requests acceptance for the deposit.
       The Bridge checks the deposit (revealed, reserved, routed to the
       vault, not swept, no duplicate position or pending action), checks
       that the named wallet is the designated wallet and is Live, checks
       the amount against the minimum and the caps, and records a pending
       authorization: a deadline, snapshots of the max fee, the minimum
       amount, the term and the dissolution delay. Capacity is reserved.
  2. DESIGNATED WALLET, on Bitcoin: the wallet's signers check the
       authorization on-chain, wait for the deposit to be confirmed, and
       sign a one-input/one-output anchor transaction that pays the
       wallet's key.
  3. SPV MAINTAINER, on Ethereum: proves the confirmed anchor transaction
       to the Bridge (Merkle inclusion, coinbase proof, proof-of-work).
  4. BRIDGE, on Ethereum: settles the action - writes the position (owner,
       custodying wallet, anchor outpoint, claim equal to anchor amount,
       state Active, expiry and dissolution dates recorded) and credits
       the vault. The vault mints tBTC against the anchor and transfers it
       to the owner, minus the initiation fee.
```

Only the depositor may request acceptance. The signing window is bounded: the deposit must be at least 2 hours old, signers wait 6 Bitcoin confirmations before proposing, and the window must fit before the refund deadline minus a 24-hour safety margin. Parameters that matter for proof and settlement are snapshotted at request time; later governance changes do not affect the in-flight action or the dates already recorded.

### 4. Custody

The position is Active. The owner holds ordinary tBTC: the anchor amount minus the initiation fee. The anchor sits under the custodying wallet; the wallet's honest majority protects it, fraud challenges police it, and the Bridge keeps the tracked claim equal to the anchor at all times. Re-anchoring is available at any position age. The recorded term dates exist so v2 can honor the term; in M1 nothing expires, renews or closes on them, except that a position still sitting on a Closing wallet can be stranded once its dissolution date has passed.

### 5. Re-anchor when a wallet retires

```
  1. REQUEST, on Ethereum: the position must be Active. If the source wallet
     is Live, only governance may request (deliberate rotation off a healthy
     wallet). If the source is retiring (MovingFunds or Closing), anyone may
     request. The target must be a different wallet in the Live state, with a
     free slot under its caps. Amount floor: the anchor must be greater than
     the minimum plus one max fee.
  2. SIGN, on Bitcoin: the source wallet's signers check the authorization
     and sign a one-input/one-output re-anchor transaction paying the
     target wallet. When a wallet is moving its funds, signer software
     requests and signs these re-anchors automatically.
  3. PROVE, on Ethereum: an SPV maintainer proves the transaction. The Bridge
     moves custody to the target, shrinks the claim by the fee, releases the
     source wallet's slot, and asks the vault to finance the fee: the vault
     burns tBTC from its reserve, and any shortfall becomes public,
     repayable fee debt.
  4. LATE PROOF: a signed, confirmed re-anchor can be proven with no time
     limit; a late proof can even restore a position that was stranded in
     the meantime onto the new wallet.
  IF THE REQUEST TIMES OUT: anyone can file the timeout. The target slot is
  released, the position returns to Active on the same source wallet, and a
  cooldown applies to the next non-governance request. No one is slashed.
```

Once a retiring wallet holds no reservations and its remaining balance is below the dust threshold, signer software reports that, so the wallet can begin closing.

### 6. How a position ends in M1

Stranding is M1's only terminal state. Anyone may file it when the position is Active and the custodying wallet is Terminated, Closed, or Closing with its dissolution date passed. The Bridge releases the capacity, drops the anchor from tracking, and records the position as Stranded. The owner's tBTC balance does not change; what is lost is the in-kind option - getting that specific bitcoin back, which v2 can honor. There is no compensation mechanism: if the bitcoin is really gone, the shortfall affects tBTC backing as a whole, exactly as with any terminated tBTC wallet today.

***

## Bitcoin transactions

```
  ANCHOR TRANSACTION (acceptance)
  +-------------------------------------+
  | input 0:  the reserved deposit UTXO |
  | output 0: P2WPKH, the wallet's key  |
  +-------------------------------------+
  value rules (proof-time):
    deposit - output <= max fee
    output >= minimum amount

  RE-ANCHOR TRANSACTION
  +-------------------------------------+
  | input 0:  the current anchor        |
  | output 0: P2WPKH, the target wallet |
  +-------------------------------------+
  value rules (proof-time):
    anchor - output <= max fee
    output > max fee (dust floor)
  (the request itself required: anchor > minimum + max fee)

  LINEAGE:
  deposit UTXO -> anchor on wallet A -> anchor on wallet B -> ...
  every transaction spends exactly the output created by the previous
  one, so any Bitcoin node or explorer can re-walk the whole chain
  from the original deposit.
```

Both shapes are enforced at proof time by the Bridge: exactly one input, exactly one output, the output paying the authorized wallet's key, fee within the snapshotted maximum, and the value floor for that transaction type; the minimum-plus-max-fee floor for re-anchors is checked when the re-anchor is requested. The signer software builds P2WPKH outputs. A Bitcoin node can check that the signature belongs to the wallet's key, but it cannot check anything more: the one-input/one-output shape and the value rules are Ethereum checks, applied to the proven transaction.

***

## Amounts and fees

```
  ILLUSTRATIVE NUMBERS (not the deployed parameters)

  deposit           10.0000000 BTC
    |
    |  anchoring miner fee 0.0001 BTC, paid by the owner
    v
  anchor             9.9999000 BTC
    |
    |  tBTC minted 1:1 against the anchor
    v
  tBTC minted        9.9999000 tBTC
    |
    |  initiation fee 0.40% = 0.0399996 tBTC, kept in the vault reserve
    v
  owner receives     9.9599004 tBTC

  later re-anchor, miner fee 0.0001 BTC:
  anchor 9.9999 -> 9.9998 BTC; the vault burns 0.0001 tBTC from its
  reserve; the owner's 9.9599004 tBTC is untouched.
```

- Who pays each fee: the anchoring miner fee is paid by the owner (the anchor is the deposit minus the fee, and tBTC is minted on the anchor). The initiation fee is taken from the minted amount and kept by the vault. Re-anchor miner fees are paid by the vault's fee reserve: the vault burns tBTC in step with each bitcoin fee, so total tBTC supply falls as the bitcoin is spent.
- Public fee debt: if the reserve is short, the unpaid part is recorded as public debt. While the debt is non-zero, tBTC supply exceeds backing by exactly that amount. Anyone can repay it, by burning their own tBTC. The vault owner's fee sweep repays the debt first, and can only send what is above the reserve target.
- No normal deposit treasury fee applies to reserved deposits.
- There is no lifetime cap on re-anchor fees in M1 (one is planned for v2). Each move's fee is capped by the snapshotted maximum, and a re-anchor can only be requested while the anchor exceeds the minimum plus one max fee, so re-anchor fees can never take a position below the minimum.

***

## States

```
  RESERVED DEPOSIT (before a position exists)

  +---------+  acceptance proof settles  +--------------------------+
  | Pending | -------------------------> | Accepted: now a position |
  +---------+                            +--------------------------+
       |
       | refund deadline passed, no acceptance pending: anyone
       | marks it stale (governance may force it earlier)
       v
  +---------+
  | Stale   |  refund on Bitcoin; can never be accepted or swept
  +---------+

  RESERVATION POSITION

  +---------+
  | Unknown |  no position yet; a pending acceptance
  +---------+  lives only in its action record
       |
       | acceptance proof settles
       v
  +---------+  re-anchor requested    +---------------+
  | Active  | ----------------------> | ActionPending |
  |         | <---------------------- |               |
  +---------+  re-anchor proven (now  +---------------+
       |       on the target wallet)
       |       or timed out (still on
       |       the source wallet)
       |
       | custodying wallet Terminated or Closed,
       | or Closing past the dissolution date
       v
  +----------+
  | Stranded |
  +----------+

  A late proof of an already-signed re-anchor can restore a Stranded
  position onto the new wallet. Closed is v2 only (in-kind redemption
  or dissolution); it is unreachable in M1.

  ACTION STATES (one per request): Pending, then Settled, or TimedOut
  (a late proof can still settle it), or Superseded (an older request's
  late proof used the anchor first). Vetoed is v2 only. M1 action
  types: Acceptance and Reanchor.
```

***

## When things go wrong

| Situation | Who can act | Result |
|-----------|-------------|--------|
| Acceptance not signed in time | Anyone files the timeout; the depositor can request again | Capacity released; new request with fresh nonce while refund window open; no slashing. |
| Re-anchor not completed in time | Anyone files the action timeout | Target slot released; position back to Active on same source; cooldown on next non-governance request; no slashing. |
| Deposit never accepted | Anyone marks it stale after refund deadline; governance can force it earlier | Pending record cleared; bitcoin refunded via the deposit script refund path on Bitcoin. |
| Custodying wallet terminated | Anyone files the stranding | Capacity released, position Stranded. Owner's tBTC unchanged; no compensation. |
| Proof arrives late or after wallet stopped | SPV maintainer submits it | Timed-out acceptance provable within deadline plus term; re-anchor provable anytime; wallet already Closing/Closed/Terminated means stranded. |
| The custodying wallet's signers stop responding | No one can move the anchor without them | Positions stay where they are and the owner's tBTC is unaffected. If the wallet then misses a protocol deadline (for example while moving its funds) it is slashed and terminated, and its positions can be stranded. There is no owner-held key to recover the bitcoin independently. |

The M1 reservation timeouts - acceptance and re-anchor - do not slash anyone; no stake is seized by either. Wallet-level timeouts are different and still apply: a retiring wallet that fails to move its funds in time is slashed and terminated, which is standard tBTC wallet machinery, separate from the reservation rules.

***

## Caps and parameters

All values below are set by the Threshold Council through Bridge governance: begin, governance delay, finalize. Vault fee settings change instantly, within the contract's hard cap.

| Parameter | Plain words |
|-----------|-------------|
| `reservationMinAmount` | Minimum anchor size, in satoshi. A deposit must be at least this plus the max fee, and a re-anchor can only be requested while the anchor exceeds this plus the max fee. |
| `reservationTxMaxFee` | Maximum miner fee for one anchor or re-anchor transaction. |
| `reservationTermSeconds` | The custody term of a position; the contract accepts between 90 and 730 days. |
| `reservationDissolutionDelay` | Delay after the term before dissolution can happen; a v2 feature, recorded in M1; in M1 it also sets when a position left on a Closing wallet can be stranded. |
| `reservationActionTimeout` | How long a requested action has before it times out. |
| `reservationRenewalWindowSeconds` | Renewal window; a v2 feature, must still be set. |
| `reservationMaxTotalAmount` | Global cap on the satoshi locked under active reservations; 0 turns the cap off. |
| `maxReservationsPerWallet` | How many open reservations one wallet may custody; must be greater than 0 (1 at launch). |
| `maxReservationsAmountPerWallet` | Cap on the satoshi one wallet may custody across its reservations; 0 turns it off. |
| `reservationMaxSingleAmount` | Cap on a single position; 0 turns it off. |
| `maxActiveReservations` | Global cap on open positions; must be greater than 0. |

Enforced relations: max fee > 0; minimum > max fee; term 90-730 days; renewal window > 0 and < term; action timeout > 2 hours; per-wallet count > 0; total cap <= max open positions x single cap (when both set); vault address can only change when no reservation is open and no reserved deposit is pending.

Launch posture: M1 launches with a small total cap, limited to design partners, with `maxReservationsPerWallet` set to 1 and a planned custody term of 12 months. Quotable code defaults: initiation fee 0.40% at deploy, hard-capped at 5%; minimum deposit age 2 hours; refund safety margin 24 hours; term range 90-730 days; signers wait 6 Bitcoin confirmations before proposing. The other values above are governance-set.

***

## Trust model

| Component | What you rely on | Enforced by |
|-----------|------------------|-------------|
| Bitcoin custody of the anchor | An honest majority of the custodying wallet's signers (51 of 100 seats on mainnet), the same as pooled tBTC | Threshold ECDSA; fraud challenges and stake slashing |
| Correct reservation records and minting | Bitcoin and Ethereum consensus only | The Bridge checks the SPV proof and the exact transaction shape before any state change |
| Segregation from pooled funds | Nothing extra | The Bridge rejects sweeps containing reserved deposits; signers refuse them too |
| Deposit before acceptance | Nothing extra | The standard tBTC deposit refund path on Bitcoin after the refund locktime |
| Proof delivery | Authorized SPV maintainers, and the Bridge's Bitcoin relay being kept up to date | Liveness only: maintainers cannot forge a proof |
| Moving positions off retiring wallets | tBTC signer software doing its duties; free slots on Live wallets | Permissionless requests, caps, timeouts |
| Housekeeping (timeouts, stale deposits, stranding) | Anyone | Permissionless calls, automated by signer software |
| Caps and parameters | The Threshold Council | Bridge governance with delay; snapshots protect in-flight actions |
| Vault fees | The Threshold Council as vault owner | Instant changes, contract cap of 5% |

**Net trust model:** M1 keeps tBTC v2's custody model. What it adds is segregation and a traceable, proven lineage for each deposit. There is no depositor-held key and no Bitcoin Script recovery path after acceptance.

***

## M1 limits

1. No in-kind redemption yet. In M1 the owner holds, transfers or uses tBTC, or redeems through standard (pooled) tBTC redemption, which pays from pooled wallet funds, not the anchor.
2. No owner-initiated way to close a position in M1: no redemption, renewal or dissolution. It stays open until v2 or until its wallet dies (Stranded).
3. The term is recorded, not enforced. Expiry and dissolution dates exist so v2 can honor the term; in M1 their only effect is the stranding gate on Closing wallets.
4. Stranding: if the custodying wallet is terminated, the in-kind option is lost; the tBTC balance is unchanged; no compensation exists. If that wallet's bitcoin is really lost, the shortfall affects tBTC backing as a whole.
5. Moving positions depends on the network: re-anchor needs a free slot on another Live wallet and working signer software; off a healthy Live wallet only governance can move a position.
6. Re-anchor fees shrink the anchor, not the owner's tBTC; covered by the vault's fee reserve, with shortfalls becoming public repayable debt. No lifetime fee cap in M1; positions at or below minimum plus one max fee can never be re-anchored. Size deposits well above the minimum.
7. Capacity is checked at acceptance request, not when bitcoin is sent. Check free slots and caps first; unaccepted deposits refund on Bitcoin after their locktime.
8. Launch is limited: design partners, small caps, one reservation per wallet.
9. Fees and parameters are set by the Threshold Council. Vault changes are instant (capped at 5%); Bridge changes use the governance delay and never affect in-flight actions or recorded dates.

***

## Verify a reservation yourself

Everything above is public state; no special tooling is required.

1. Compute the reservation key: the standard tBTC deposit key - keccak256 of the funding transaction hash and output index, as the Bridge uses for every deposit.
2. Read the Bridge address: `reservations(key)` gives owner, wallet, anchor, amounts, state and dates; `reservationActions(key, nonce)` for pending actions; `reservationParameters()` for operational parameters; `reservationCaps()` for the per-wallet amount, single-position, and max-open-positions caps; `activeReservationsCount()` for current open count; `walletReservationsCount(hash)` and `walletReservationsAmount(hash)` per wallet; `pendingReservedDeposits()` and `reservedDepositWallet(key)` pre-acceptance; `reservationByAnchorUtxo(txHash, 0)` finds a position from its bitcoin outpoint.
3. Check the anchor output on any Bitcoin node or explorer: it must pay the custodying wallet's key at the recorded amount.
4. Follow the events: `ReservationAcceptanceRequested`, `ReservationAccepted`, `ReservationReanchorRequested`, `ReservationReanchored`, `ReservationAcceptanceTimedOut`, `ReservationReanchorTimedOut`, `ReservationLateSettled`, `ReservationStranded`, `ReservedDepositMarkedStale`.
5. Read the vault: `initiationFeeBps`, `feeReserveTarget`, and `inKindFeeDebtSat` (the outstanding public fee debt).

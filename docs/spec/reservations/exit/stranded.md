# Reservation Stranding — the Existing Fallback

Status: LIVE design, source-verified against `feat/utxo-reservation-guards` (#1094). This is what
already exists and already ships, and under the Decision in `README.md` (2026-08-21) it is the
**accepted** fallback — the emergency-exit family (`proposal.md`, `alternatives.md`,
`addendum.md`) explored replacing it but is deferred for lack of evidence, so `Stranded` stands.
Companion to `README.md` (where `Stranded` sits in
the comparison), `../reservations-feature-spec.md` §7 H-06 (the two-paragraph original spec), and
`../reservations-stranding-compensation-proposal.md` (the compensation module this feature
deliberately does not build).

## 1. Plain explanation

`Stranded` is a write-off, not a rescue. It is the accounting step that admits a reservation's
Bitcoin is no longer reachable through the protocol and stops pretending otherwise. It does not
move money, does not change anyone's tBTC balance, and does not compensate anyone.

**Two preconditions, both required:**

1. The custodying wallet is `Terminated`.
2. The reservation itself is `Active` — no action mid-flight. A reservation with a pending
   redemption, re-anchor, or dissolution cannot be stranded directly; the pending action must
   settle, get vetoed, or time out first. All three resolutions are permissionless and need no
   cooperation from the (already dead) wallet, so a pending action can never permanently block
   stranding — it only delays it by however long is left on that generation's timeout.

**`Terminated` has three unrelated causes, and only one of them is malice.** A wallet's
termination has nothing to do with whatever reservations it happens to be custodying at the time —
it is purely a property of that wallet's own history:

- **`notifyWalletMovingFundsTimeout`** — a wallet told to move its funds (after a redemption
  timeout or heartbeat failure pushed it into `MovingFunds`) failed to complete the move before
  its own timeout. Pure liveness failure: operators offline, infra trouble, a bug — no malice
  required.
- **`notifyWalletMovedFundsSweepTimeout`** — a wallet that was asked to prove it swept funds moved
  in from another wallet failed to produce that proof in time. Same category: a liveness failure,
  and it terminates the *receiving* wallet, which may separately be custodying reservation anchors
  of its own, unrelated to the sweep it failed to prove.
- **`notifyWalletFraudChallengeDefeatTimeout`** — a fraud challenge was raised against the wallet
  (it signed something unauthorized) and it failed to defeat the challenge before the timeout.
  This is the only path that implies deliberate malice.

Each path seizes a different named slashing amount from the operators' stake
(`movingFundsTimeoutSlashingAmount`, `movedFundsSweepTimeoutSlashingAmount`, or
`fraudSlashingAmount`) and rewards whoever reported it, then calls the same `terminateWallet`.
None of the three moves any Bitcoin. **This matters for what "the loss" actually is:** in the
fraud case, the wallet is adversarial and its BTC is plausibly genuinely stolen or otherwise
gone. In the two timeout cases, the operators may be entirely honest and the BTC may still sit
untouched at the anchor's address — they simply failed to act inside a deadline. Either way,
`Terminated` is a **one-way** state with no path back to `Live`, so the protocol will never again
trust a signature from that wallet, regardless of which of the three caused it. A timeout
termination therefore converts "temporarily unresponsive" into "permanently unrecoverable" purely
as a matter of protocol policy, not because the Bitcoin itself became unspendable.

**What the call does — `notifyReservationStranded(reservationKey)`, permissionless, no reward:**

1. Flips reservation state `Active → Stranded`.
2. Releases tracked capacity: decrements the wallet's `walletReservationsCount` /
   `walletReservationsAmount` and the global `reservationTotalAmount` by the anchor's value.
3. Removes the reservation from the wallet's enumeration list and the anchor→reservation reverse
   index.
4. Emits `ReservationStranded(reservationKey, walletPubKeyHash, owner, anchorAmount)` — the only
   artifact. No storage record naming an owed amount is created; the event is the entire trail.

The anchor is **deliberately not marked as honestly spent** — a terminated wallet's spend of it
stays recognizable as such in the fraud/dispute record, rather than being reclassified as a normal
settlement.

**What does not happen: the depositor's tBTC balance never changes.** They received
`mintedAmount` in tBTC at acceptance time, long before any of this. That tBTC is already theirs,
already spendable, and is completely unaffected by whether the reservation is `Active` or
`Stranded`. What stranding actually removes is the *option* — the right to redeem those exact
coins back in-kind rather than through the shared pool. After stranding, the owner is an ordinary
tBTC holder: no better, no worse than a depositor who never reserved anything.

**Termination is never gated on reservations; voluntary closing is.** All three termination paths
above have no reservation check at all, by design — punitive or timeout-driven termination must
never be blockable by an unrelated position. The *voluntary* wind-down path
(`beginWalletClosing` / `finalizeWalletClosing`) is the opposite: both require
`walletReservationsCount == 0` before a wallet may close, forcing every live anchor to be
redeemed, re-anchored, or dissolved first. `Stranded` exists precisely to handle the case that
guard cannot: a wallet that never got the chance to wind down honestly.

## 2. Step-by-step example
84:
85:The representative case is a liveness failure, not fraud — moving-funds timeouts require no
86:malice and are the more ordinary way a wallet ends up `Terminated`.
87:
88:1. **Alice reserves.** She deposits 5 BTC into a reservation custodied by wallet W1. At
89:   acceptance: `mintedAmount = anchorAmount = 5 BTC`, 5 tBTC minted to Alice, reservation state
90:   `Active`.
91:2. **W1 ages out and is asked to move funds** — nothing to do with Alice's reservation; W1 is
92:   simply old enough (or low enough on non-reservation balance) to be rotated out in the normal
93:   course of wallet lifecycle. `moveFunds` transitions it `Live → MovingFunds`.
3. **The escape hatch exists.** While W1 is in `MovingFunds`, Alice's reservation *can* be
   re-anchored to any Live wallet with capacity (`requestReservationReanchor` is explicitly
   allowed for `MovingFunds` source wallets, `../reservations-feature-spec.md` §4.3). If the
97:   keep-core executor or Alice initiates this, the anchor migrates cleanly to a healthy wallet
98:   and Alice keeps her in-kind claim — stranding is avoided entirely.
99:4. **W1's operators go quiet** — infrastructure trouble, an upgrade gone wrong, insufficient
100:   uptime, no theft implied — and fail to complete the funds move before `movingFundsTimeout`
101:   elapses. **Critically: this means re-anchor was available for the whole MovingFunds window
102:   and was not used** — the wallet went dark enough to sign nothing at all, not even the
103:   re-anchor that would have saved Alice's claim.
104:5. **Termination fires, unconditionally.** Anyone calls `notifyWalletMovingFundsTimeout`;
105:   `movingFundsTimeoutSlashingAmount` is seized from W1's operator stake and split with the
106:   caller as a reward. W1 → `Terminated`. This transition has no dependency on Alice's
107:   reservation whatsoever — it fires whether W1 holds zero reservations or fifty, and W1's
108:   operators may not have done anything dishonest at all.
109:6. **The gap.** Alice's reservation still reads `Active` in storage. Her 5 tBTC balance is
110:   unaffected either way, both now and forever — nothing about it depends on this step. The 5 BTC
111:   sitting at the anchor's address may well still be intact; what changed is that the protocol
112:   will never again accept a signature from W1, so it is permanently unreachable through the
113:   protocol regardless of whether the coins themselves are spendable.
114:7. **Someone calls `notifyReservationStranded`** — a keep-core watchtower bot, Alice herself, a
115:   bystander running a script; nobody is paid to do it and nobody is blocked from doing it.
116:   Reservation → `Stranded`. W1's and the global reservation-capacity counters drop by 5 BTC.
117:   Alice's entry is removed from W1's enumeration and the anchor index.
118:   `ReservationStranded(key, W1, Alice, 5 BTC)` fires.
119:8. **End state.** Alice still holds exactly 5 tBTC — untouched by any of the above. What she lost
120:   in step 5, not step 7, is the option to redeem those specific 5 BTC back in-kind. She can still
121:   redeem 5 tBTC through the ordinary pooled path, against backing that is now short by W1's
122:   unreachable 5 BTC — a shortfall spread across every tBTC holder, not billed to Alice alone. No
123:   compensation is paid to her specifically for losing the option. The event is the only durable
124:   record this happened to her, and nothing in that event or anywhere else distinguishes this
125:   ordinary-timeout case from a genuine theft — both look identical downstream of `Terminated`.
## 3. What is genuinely incomplete, and what to do about it
127:
128:Five items. The first three are mechanical documentation/monitoring fills, made below. The other
129:two are flagged, not silently decided.
130:
131:### 3.1 Monitoring watch-list gap (fixed here)
132:
133:`../reservations-feature-spec.md`'s executor-monitoring bullet lists `pendingReservedDeposits`,
134:`inKindFeeDebtSat`, dissolution-eligible positions, and per-wallet reserved amount/count — it does
135:not mention watching for terminated wallets that still custody un-stranded reservations. Every
136:other permissionless housekeeping call in this feature (dissolution, action timeout, marking a
137:stale deposit) is unrewarded and relies on exactly this kind of bot-driven monitoring; this one
138:case was left off the list. **Recommendation, consistent with the existing pattern:** the
139:keep-core wallet-side executor should watch for `Terminated` wallets against its own set of
140:tracked reservation keys and call `notifyReservationStranded` for each, the same way it already
141:drives dissolution and timeout calls. No protocol change, no new incentive mechanism — just
142:closing a documentation gap so the responsibility is actually assigned somewhere.
143:
144:### 3.2 Re-anchor on wallet rotation gap (fixed here)
145:
146:The spec lists `requestReservationReanchor` as an executor duty (§13), but nowhere says *when* the
147:executor should call it for a wallet entering `MovingFunds`. If the executor does not automatically
148:re-anchor all reservations to a Live target the moment its wallet transitions `Live → MovingFunds`,
149:every routine rotation of an anchor-holding wallet becomes a stranding candidate. **Recommendation:**
150:on detecting `WalletMovingFunds(walletPubKeyHash)`, the keep-core executor should enumerate that
151:wallet's reservation keys, pick a Live target with capacity, and call
152:`requestReservationReanchor` for each — the same executor that already drives dissolution,
153:timeout, and stale-deposit calls. This is purely operational assignment, no protocol change.
154:
155:### 3.3 No caller incentive exists (assumption made, flagged for override)
stale deposit) is unrewarded and relies on exactly this kind of bot-driven monitoring; this one
case was left off the list. **Recommendation, consistent with the existing pattern:** the
keep-core wallet-side executor should watch for `Terminated` wallets against its own set of
tracked reservation keys and call `notifyReservationStranded` for each, the same way it already
drives dissolution and timeout calls. No protocol change, no new incentive mechanism — just
closing a documentation gap so the responsibility is actually assigned somewhere.

### 3.2 No caller incentive exists (assumption made, flagged for override)

Delay in calling `notifyReservationStranded` has exactly one consequence: the stranded anchor's
BTC amount keeps occupying a slot against `reservationMaxTotalAmount`, the *global* reservation
cap, until someone calls it — wasting capacity, not endangering any depositor's balance (§1). The
default taken here is **no new reward mechanism** — every other permissionless notify-call in this
feature (dissolution, timeout, stale-deposit marking) already carries no reward and already relies
on ops-bot monitoring, so adding one here would be an inconsistent one-off. If un-stranded global
capacity turns out to matter in practice (e.g. the cap fills with dead anchors during a period of
neglect), the fix is operational — add it to the executor's watch-list per §3.1 — not a protocol
incentive. Flag this if a different call is wanted.

### 3.3 Stranding frequency is dominated by liveness failures, not fraud — and that is the number the rest of this folder needs

Two of the three termination paths (§1) require no malice at all: a wallet simply failed to
complete a routine funds move or sweep proof in time. Fraud requires operators to deliberately
sign something unauthorized, knowing they will be publicly caught and slashed — a rarer, higher-
stakes event than ordinary operational downtime. **Expected annual stranding frequency, the
number `README.md`'s Open Items already tracks as decisive for whether Mechanism 1 is worth its
standing cost, should therefore be modeled primarily against operator uptime/liveness statistics,
not against an assumed fraud rate.** This also changes what an emergency exit would actually be
recovering in the common case: for a moving-funds or sweep-timeout termination, the underlying BTC
may still be sitting untouched at the anchor's address, reachable in principle if a depositor had
an alternate co-signing path that did not depend on the wallet's own (merely slow, not dishonest)
key. That is a stronger case for Mechanism 1 than "rescuing coins that are probably already
stolen" — a meaningful share of terminations may not represent an actual Bitcoin-layer loss at
all, only a protocol-policy write-off.

### 3.4 The fraud-griefing property (documented, not a design defect to fix here)

The fraud path specifically is operator-triggerable: since fraud requires the wallet's own
operators to sign something unauthorized, that operator set can *choose* to strand any reservation
it custodies — commit fraud, eat `fraudSlashingAmount`, and let the depositor's segregated,
in-kind claim collapse into the shared pooled write-off while walking off with the anchor's actual
BTC. **This is not a vulnerability specific to reservations** — the same operators could already
walk off with any pooled wallet's main UTXO the same way, and either theft becomes the same kind
of network-wide socialized shortfall once it happens. What reservations change is *who the victim
is*: a pooled wallet's stolen main UTXO loss is diffused anonymously across whoever happened to be
backed by that wallet; a stranded reservation's lost option is billed to one named, individually
identifiable depositor. Whether that concentration is profitable for a colluding operator set
depends on stake-at-risk versus anchor value, which is unquantified (§3.3). This is exactly the
failure mode `proposal.md`'s Mechanism 1 targets, *provided* the depositor armed their exit before
the wallet went bad — armed-after-the-fact protection does not exist, and cannot: fraud is not
announced in advance by design, so a depositor with no live escrow arrangement has no way to
evacuate a reservation in response to a wallet turning hostile. This is not a gap in `Stranded` to
close; it is the reason the rest of this folder exists.

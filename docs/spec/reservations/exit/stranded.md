# Reservation Stranding — the Existing Fallback

Status: implemented in the M1 code (`reservations-upgrade` @ `9f8f5ef1`) — not yet merged to
`dev`/`main`, not audited, not deployed. This document verifies `notifyReservationStranded` and
its accounting (`strandReservation`, `releaseAcceptanceCapacity`) against that code, plus
`pkg/maintainer/spv/reservation_stranding_watch.go` in keep-core (`reservations-epic` @
`f66f11240`) for the executor side. Under the Decision in `README.md` (2026-08-21) it is the
reservation feature's only terminal path and the **accepted** fallback — the emergency-exit
family (`proposal.md`, `alternatives.md`, `addendum.md`) explored replacing it but is deferred
for lack of evidence, so `Stranded` stands. Companion to `README.md` (where `Stranded` sits in
the comparison), `../feature-spec.md` §7 H-06 (the two-paragraph original spec), and
`../stranding-compensation-proposal.md` (the compensation module this feature deliberately does
not build).

## 1. Plain explanation

`Stranded` is a write-off, not a rescue. It is the accounting step that admits a reservation's
Bitcoin is no longer reachable through the protocol and stops pretending otherwise. It does not
move money, does not change anyone's tBTC balance, and does not compensate anyone.

**Two preconditions, both required:**

1. The custodying wallet is in a state from which it can never sign again: `Terminated`,
   `Closed`, or `Closing` once the reservation's own `dissolutionEligibleAt` has passed
   (`block.timestamp >= reservation.dissolutionEligibleAt`) — `Reservation.sol`
   `notifyReservationStranded`, lines ~1090-1115. `Closing` is excluded before that deadline
   because `requestReservationReanchor` accepts a `Closing` source wallet; letting the same
   wallet be stranded early would race a legitimate reanchor against a permissionless strand.
   After the deadline both become independently callable, but only one lands: both share the
   `reservation.state == Active` precondition below, so whichever transaction confirms first
   flips it and the other reverts.
2. The reservation itself is `Active` — no action mid-flight. M1 has exactly one action type
   that can leave a reservation `ActionPending`: re-anchor (`requestReservationReanchor`,
   `Reservation.sol` ~833). `Redemption`, `Dissolution`, and the `Vetoed` action-state are
   declared in the same enums but are not reachable through any M1 router entry point or code
   path — kept only for m2 storage/layout stability (`Reservation.sol` ~109-110, ~138). So a
   reservation with a pending re-anchor cannot be stranded directly; that action must settle or
   time out first. Both resolutions need no cooperation from the (already dead) wallet, but
   they are not equally permissionless: timeout (`notifyReservationActionTimeout`) carries no
   caller restriction, while settlement needs an SPV proof submitted via
   `submitReservationReanchorProof`, gated `onlySpvMaintainer` (`ReservationRouter.sol`
   ~194-198, ~283) to a governance-allowlisted set of addresses (`Bridge.sol`
   `setSpvMaintainerStatus`, `onlyGovernance`) rather than to literally anyone. Either way, a
   pending re-anchor can never permanently block stranding — it only delays it by however long
   is left on that generation's timeout, or until an allowlisted maintainer submits the
   settlement proof.

**`Closed` and dissolution-eligible `Closing` reach this precondition through a different,
non-malicious gap than `Terminated` below.** Two of `beginWalletClosing`'s three callers do not
check whether the wallet still custodies reservations before starting the closing clock: a
`MovingFunds` wallet that proves its Bitcoin funds move (`notifyWalletFundsMoved`) or whose main
UTXO drops below the moving-funds dust threshold (`notifyWalletMovingFundsBelowDust`) both begin
`Closing` unconditionally (`Wallets.sol` ~389-460); only the *immediate*-closing branch inside
`moveFunds` itself checks `walletReservationInfo[wallet].count == 0` first. So a wallet can enter
`Closing` while still holding an open reservation that was never re-anchored away in time. The
check that cannot be skipped is at the other end: `notifyWalletClosingPeriodElapsed`
hard-requires `walletReservationInfo[wallet].count == 0` before `finalizeWalletClosing` can move
the wallet to `Closed` (`Wallets.sol` ~378-384). In practice this means a `Closed` wallet has
already had every reservation resolved — the `Closed` branch of precondition 1 is chiefly a
defensive completeness case standing alongside the genuinely load-bearing `Closing` one, which
is the real second path into `Stranded` beyond `Terminated`.

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

1. Flips reservation state `Active -> Stranded`.
2. Releases tracked capacity via `releaseAcceptanceCapacity`: decrements the wallet's
   `walletReservationsCount` by one and `walletReservationsAmount` by the anchor's value,
   decrements the global `reservationTotalAmount` by the anchor's value, and decrements the
   global `activeReservationsCount` by one — emitting `ReservationOccupancyChanged
   (activeReservationsCount)` unconditionally as part of that same internal call.
3. Removes the reservation from the anchor->reservation reverse index
   (`reservationsByAnchorUtxo`) by deleting that mapping entry. There is no separate per-wallet
   enumeration list in the M1 storage layout: `walletReservationKeys`/`walletReservationKeyIndex`
   were identified as dead storage and removed in the milestone rewrite (`BridgeState.sol`
   ~494-497). Step 2's aggregate count/amount is the entirety of wallet-level bookkeeping.
4. Emits `ReservationStranded(reservationKey, walletPubKeyHash, owner, anchorAmount)` — a second
   event, distinct from step 2's `ReservationOccupancyChanged`, and the only one naming the owed
   amount. No storage record naming an owed amount is created; this event is the entire recovery
   trail.

The anchor is **deliberately not marked as honestly spent** — a terminated wallet's spend of it
stays recognizable as such in the fraud/dispute record, rather than being reclassified as a normal
settlement.

**What does not happen: the depositor's tBTC balance never changes.** They received
`mintedAmount` in tBTC at acceptance time, long before any of this. That tBTC is already theirs,
already spendable, and is completely unaffected by whether the reservation is `Active` or
`Stranded`. What stranding actually removes is the *option* — the right to redeem those exact
coins back in-kind rather than through the shared pool. After stranding, the owner is an ordinary
tBTC holder: no better, no worse than a depositor who never reserved anything.

**Termination is never gated on reservations; only the wallet's final closure is — and even
that gate has a gap.** All three termination paths above have no reservation check at all, by
design: punitive or timeout-driven termination must never be blockable by an unrelated position.
Voluntary closing is different, but not uniformly gated the way it might look: as walked through
in §1 above, two of the three routes into `Closing` skip the reservation check entirely, and only
finalizing `Closing` into `Closed` hard-requires `walletReservationsCount == 0`. `Stranded` exists
precisely to handle the gap that leaves open: a reservation still `Active` on a wallet that
started closing without one, and will never sign again.

## 2. Step-by-step example

The representative case is a liveness failure, not fraud — moving-funds timeouts require no
malice and are the more ordinary way a wallet ends up `Terminated`.

1. **Alice reserves.** She deposits 10 BTC into a reservation custodied by wallet W1. At
   acceptance: `mintedAmount = anchorAmount = 10 BTC`, 10 tBTC minted to Alice, reservation state
   `Active`.
2. **W1 ages out and is asked to move funds** — nothing to do with Alice's reservation; W1 is
   simply old enough (or low enough on non-reservation balance) to be rotated out in the normal
   course of wallet lifecycle. `moveFunds` transitions it `Live -> MovingFunds`.
3. **The escape hatch exists.** While W1 is in `MovingFunds`, Alice's reservation *can* be
   re-anchored to any Live wallet with capacity (`requestReservationReanchor` is explicitly
   allowed for `MovingFunds` source wallets, `../feature-spec.md` §4.3). If the
   keep-core executor or Alice initiates this, the anchor migrates cleanly to a healthy wallet
   and Alice keeps her in-kind claim — stranding is avoided entirely.
4. **W1's operators go quiet** — infrastructure trouble, an upgrade gone wrong, insufficient
   uptime, no theft implied — and fail to complete the funds move before `movingFundsTimeout`
   elapses. **Critically: this means re-anchor was available for the whole MovingFunds window
   and was not used** — the wallet went dark enough to sign nothing at all, not even the
   re-anchor that would have saved Alice's claim.
5. **Termination fires, unconditionally.** Anyone calls `notifyWalletMovingFundsTimeout`;
   `movingFundsTimeoutSlashingAmount` is seized from W1's operator stake and split with the
   caller as a reward. W1 -> `Terminated`. This transition has no dependency on Alice's
   reservation whatsoever — it fires whether W1 holds zero reservations or fifty, and W1's
   operators may not have done anything dishonest at all.
6. **The gap.** Alice's reservation still reads `Active` in storage. Her 10 tBTC balance is
   unaffected either way, both now and forever — nothing about it depends on this step. The 10 BTC
   sitting at the anchor's address may well still be intact; what changed is that the protocol
   will never again accept a signature from W1, so it is permanently unreachable through the
   protocol regardless of whether the coins themselves are spendable.
7. **Someone calls `notifyReservationStranded`** — the keep-core stranding watcher
   (`reservationStrandingWatcher`, wired to wallet close/termination events), Alice herself, a
   bystander running a script; nobody is paid to do it and nobody is blocked from doing it.
   Reservation -> `Stranded`. W1's `walletReservationsCount`/`walletReservationsAmount` and the
   global `reservationTotalAmount`/`activeReservationsCount` all drop (by one count and 10 BTC of
   amount, respectively), emitting `ReservationOccupancyChanged`. Alice's entry is removed from
   the anchor -> reservation reverse index (there is no separate per-wallet list).
   `ReservationStranded(key, W1, Alice, 10 BTC)` fires as the second event — the one that names
   the amount.
8. **End state.** Alice still holds exactly 10 tBTC — untouched by any of the above. What she lost
   in step 5, not step 7, is the option to redeem those specific 10 BTC back in-kind. She can still
   redeem 10 tBTC through the ordinary pooled path, against backing that is now short by W1's
   unreachable 10 BTC — a shortfall spread across every tBTC holder, not billed to Alice alone. No
   compensation is paid to her specifically for losing the option. The event is the only durable
   record this happened to her, and nothing in that event or anywhere else distinguishes this
   ordinary-timeout case from a genuine theft — both look identical downstream of `Terminated`.

## 3. What is genuinely incomplete, and what to do about it

Five items remain relevant, though two turned out not to be gaps at all. §3.1 and §3.2 are
already covered elsewhere — kept below only as pointers to where. §3.3 is a genuine
documentation fill: an assumption made here, not elsewhere. §3.4 and §3.5 are flagged, not
silently decided.

### 3.1 Monitoring watch-list — one gap remains

`../feature-spec.md`'s executor-monitoring bullets (lines ~928-941) already assign the duty to
watch `Terminated` wallets still custodying un-stranded reservations and call
`notifyReservationStranded` for each. keep-core already implements it:
`pkg/maintainer/spv/reservation_stranding_watch.go`'s `reservationStrandingWatcher`, wired to
wallet close/termination events, walks a wallet's custodied reservations
(`checkReservationStrandingForWallet`) and forwards a stranding notification for every one still
`Active`, retrying transient RPC failures up to three times.

**The one remaining gap (`../m1-keep-core-readiness/01-gap-analysis.md`'s "Stranding coverage
gap for `Closing`-past-`dissolutionEligibleAt` wallets" row):** the watcher acts only on wallets in
`StateClosed` or `StateTerminated`. The contract also allows stranding a reservation on a
`Closing` wallet once `now >= reservation.dissolutionEligibleAt` (`Reservation.sol`
~1104–1107), but no client path triggers that condition. `ReservationReanchorTask`
(`pkg/tbtcpg/reservation_reanchor.go ~:139`) drains MovingFunds sources but does not
handle Closing wallets. The permissionless calls (`notifyReservationStranded`,
`requestReservationReanchor`) are the backstop for this case; closing the gap in the
automated watcher is a keep-core task item. This subsection is retained as a pointer to
the implementation and the gap.

### 3.2 Re-anchor on wallet rotation — already covered, not a gap

`../feature-spec.md` already specifies exactly this timing: "On `Live -> MovingFunds`, re-anchor
every open reservation to a Live target... it must initiate re-anchor promptly on
`WalletMovingFunds(walletPubKeyHash)`, not wait for an expiry signal" (lines ~919-927). keep-core
already implements it: `pkg/tbtcpg/reservation_reanchor.go`'s `ReservationReanchorTask` is
triggered once a wallet enters `StateMovingFunds` (or falls below the moving-funds dust
threshold) and, for every reservation the wallet still custodies, picks a Live target and
assembles a re-anchor transaction. There is no remaining gap; this subsection is retained only as
a pointer to that implementation.

### 3.3 No caller incentive exists (assumption made, flagged for override)

Delay in calling `notifyReservationStranded` has exactly one consequence: the stranded anchor's
BTC amount keeps occupying a slot against `reservationMaxTotalAmount`, the *global* reservation
cap, until someone calls it — wasting capacity, not endangering any depositor's balance (§1). The
default taken here is **no new reward mechanism** — every other permissionless notify-call in
this feature (action timeout, stale-deposit marking; `dissolution` is an m2-only action type not
reachable in this code, see §1) already carries no reward and already relies on ops-bot
monitoring, so adding one here would be an inconsistent one-off. If un-stranded global capacity
turns out to matter in practice (e.g. the cap fills with dead anchors during a period of
neglect), the fix is operational and already largely in place — the stranding watcher (§3.1)
already sweeps for exactly this; tightening its cadence, not adding a protocol incentive, is the
lever. Flag this if a different call is wanted.

### 3.4 Stranding frequency is dominated by liveness failures, not fraud — and that is the number the rest of this folder needs

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

### 3.5 The fraud-griefing property (documented, not a design defect to fix here)

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
depends on stake-at-risk versus anchor value, which is unquantified (§3.4). This is exactly the
failure mode `proposal.md`'s Mechanism 1 targets, *provided* the depositor armed their exit before
the wallet went bad — armed-after-the-fact protection does not exist, and cannot: fraud is not
announced in advance by design, so a depositor with no live escrow arrangement has no way to
evacuate a reservation in response to a wallet turning hostile. This is not a gap in `Stranded` to
close; it is the reason the rest of this folder exists.
# Reservation Emergency Exit - Design Overview

Status: DESIGN REFERENCE, deferred - not scoped in any of the 9 reservation PRs, not planned.
The decision and its re-open conditions are below. Companion to
`../feature-spec.md` (what the PRs actually build) and
`../stranding-compensation-proposal.md` (the loss-absorption question this family
deliberately does not answer).

## Decision (2026-08-21): not built - `Stranded` remains the fallback

**Decided:** do not build Mechanism 1 or any emergency-exit mechanism. The existing `Stranded`
path (`stranded.md`) remains the reservation fallback for a terminated wallet. This entire folder
is retained as **design reference** - the requirements, the two standing walls, and the mechanism
spec stay because they are the cheapest way to rebuild if the decision ever flips - but nothing
here is scoped, funded, or planned.

**Why, plainly:** every viable mechanism costs standing operational overhead - a second
threshold-signing committee with its own DKG, resharing, monitoring and governance - to cover a
failure whose real-world frequency and size are unmeasured. Two load-bearing numbers are unknown:
expected annual stranding frequency (liveness-dominant, not fraud - `stranded.md` §3.3), and how
much depositors actually value the in-kind guarantee over fungible tBTC at par. Until market/user
evidence answers those, that cost is not justified against the `Stranded` status quo, which is
free, already built, safe, and socialized. This is the same gate the analysis itself set
("worth building only if the expected annual loss justifies the cost", below).

**What would reopen it:** (1) stranding-frequency × anchor-value evidence that expected annual
loss exceeds the mechanism's carrying cost, or (2) direct market/user signals - depositors
refusing fungible tBTC at par, a competitor shipping a recoverable in-kind guarantee, or
governance pressure over lost deposits. Either reopens this folder as-is: the walls, the
mechanism spec, and the open questions are the starting point, and the one question that decides
shape (committee staffing) is already answered in `proposal.md` §5.1.

**Immediate consequence:** the compensation module
(`../stranding-compensation-proposal.md`) becomes the only buildable piece of the
broader loss story - Tiers 0-1 (liability accounting, fee restitution) - and doubles as the
evidence-gathering instrument, since Tier 0 *produces* the stranding-frequency number this
decision says is missing. The recommendation below stands unchanged: build Tiers 0-1, hold this
folder as reference.

## The problem this whole folder is about

A reservation's promise is that a depositor gets back their **specific** Bitcoin, not a fungible
share of the pool. Today, if the wallet holding that Bitcoin stops cooperating - offline, or
fraud-slashed and `Terminated` - there is no way to recover it. The existing fallback,
`Stranded`, converts the claim into an ordinary pooled tBTC balance with no bound on compensation
(mechanics and a worked example: `stranded.md`).
That is a **fungibility failure**, not a missing feature: it quietly breaks the exact guarantee
the product sells, and it is why an emergency-exit design was explored here. Whether it is *built*
is a separate question, settled by the Decision above - not yet, absent evidence.

Every mechanism here answers the same question: **once the wallet is dead, how does the depositor
get their Bitcoin out, without opening a way to take Bitcoin from a wallet that is still alive?**
Five requirements were derived to make that precise (`proposal.md` §2):

| | Requirement | Breaks if dropped |
|---|---|---|
| R1 | No periodic Bitcoin refresh dependency | Stale during perfectly healthy, multi-year custody |
| R2 | Burn is enforced, not just encouraged | Depositor sells the tBTC and still takes the BTC; shortfall lands on the buyer |
| R3 | No dependency on the failed component | Unusable in exactly the situation it exists for |
| R4 | Last resort, never a shortcut | Becomes the default exit path against healthy wallets |
| R5 | Fails closed | Its own bugs and hiccups turn into theft from a live wallet |

A later investigation (`alternatives.md`) found two more requirements that were assumed but never
written down, and that turn out to be what actually justifies the expensive parts of the design:

| | Requirement | Why it was missed |
|---|---|---|
| R6 | Exit stays subject to the redemption watchtower's live compliance/veto checks | R1-R5 all describe safety against loss; none of them describes *policy enforcement* |
| R7 | Exit can observe post-creation mutable Ethereum state (subsumes R6) | Every mechanism that removes the live party also loses this, silently |

## Three standing walls, true of the whole design space

1. **Bitcoin Script cannot verify Ethereum state.** Any mechanism with no live party at exercise
   time is, by construction, also usable by someone who never satisfied whatever Ethereum-side
   condition it meant to require. This alone kills every "just add a hash-lock / timelock"
   variant (`addendum.md` §A.2-A.3, `alternatives.md` §2.1-2.2).
2. **The exit must fail closed.** Any design that opens the Bitcoin path by default and relies on
   liveness evidence to suppress it turns the mechanism's own operational hiccups - fee spikes,
   congestion, an operator outage - into unauthorized withdrawals from healthy wallets
   (`addendum.md` §A.4).
3. **Anchor keys are immutable, so the exit's trusted party must outlive the position.** Whatever
   key the anchor script checks is fixed when the anchor is created and can never be changed.
   Among live-party designs only a *threshold group with share refresh* survives this, since it can
   change membership while keeping the group public key constant; a plain multisig cannot, and a
   single fixed signer cannot be replaced at all. This wall is what forces the machinery, and it is
   also what creates the continuity obligation in Open Items (`proposal.md` §5.1).

## The mechanisms, in one table

| # | Mechanism | Live party needed? | Verdict | Why |
|---|---|---|---|---|
| 1 | **Escrow-gated attestor/co-signer** | Yes, a new standing committee | **Deferred** | Only design that satisfies R1-R5 (R2/R4/R5 conditional on committee honesty) *and* R6/R7 - but deferred for lack of loss-frequency evidence (Decision above). Full detail: `proposal.md` |
| 2 | Ethereum-execution-derived hash-lock | No | Rejected | Ethereum has no private state; nothing is knowable-after-yet-not-precomputable. `addendum.md` §A.2 |
| 3 | Self-settling permissionless proof (static CLTV) | No | Rejected | Sell-then-wait: depositor sells the tBTC, waits out the timelock, exits for free. `addendum.md` §A.3 |
| 4 | Inverted liveness (heartbeat-suppressed exit) | No (until it fails) | Rejected | Fixes R1 but fails open: any heartbeat interruption authorizes a withdrawal from a healthy wallet. `addendum.md` §A.4 |
| 5 | Escrow-at-mint lien (`f=1`, no liquid tBTC) | No | Rejected, kept as diagnostic | Satisfies R1-R5 with no committee, but: no maximum custody term exists so `T` always eventually lapses (R4 leaks), and it bypasses the redemption watchtower (R6/R7) - the strongest argument *for* Mechanism 1. `alternatives.md` §3 |
| 5b | Partial lien (`0<f<1`) | No | Provably worse than nothing | Depositor's gain over honest exit is `1-f`, strictly positive below `f=1`; layered on a compensation backstop it doubles total system loss. `alternatives.md` §4 |
| 6 | Ad hoc single bonded co-signer (reactive, post-death) | Yes, one already-staked operator | **Rejected** | Reactive selection is not expressible: any key the anchor script checks is fixed at creation, so the co-signer cannot be chosen later. Both substitutes are worse - 1-of-`N` pre-commitment (worst-case safety) or a dormant key holder (the reactivity was illusory). `alternatives.md` §7 |
| - | Pre-signed refunds, chained pre-signing, BitVM, covenant opcodes, timelock encryption/VDFs, wallet revival via resharing, economic bonding, proportional exit, bundled position transfer | No / relocates trust | All dead | Full reasoning per family: `alternatives.md` §5 |

## Pros and cons of Mechanism 1 (retained design reference, deferred)

**Mechanism 1 - standing committee (the only live candidate; deferred per the Decision above, not adopted).**

- **Pros:** delivers full claim liquidity throughout custody, keeps a live discretionary check at
  exit time (can decline a sanctioned/vetoed owner), no hard cap on custody duration, reuses the
  same escrow-then-burn shape as normal redemption.
- **Cons:** a second, independent threshold-signing group with its own DKG, resharing, monitoring,
  membership governance and operator onboarding - and per R3 it cannot reuse the wallet's own
  operators or infrastructure, so it is not "tBTC already does this." R2/R4/R5 hold only against a
  non-colluding committee; the disarm-refund path needs the re-anchor-invalidation fix in
  `proposal.md` §4 before it is safe. And its key must stay alive for the position's entire life
  (wall 3), which is an unbounded duty while the custody term is uncapped.

Worth building only if the expected annual loss from wallet failure justifies the cost - see Open
Items below.

### The remaining design freedom is staffing, not shape

Wall 3 forces a threshold group with resharing. What is still open is *who staffs it*, and the
cheapest answer is the **Threshold Council Safe appointing and rotating a distinct group**
(`proposal.md` §5.1):

- **Why it is cheap:** the Council can already upgrade the Bridge implementation, so authorizing it
  to appoint an exit co-signer adds no marginal trust in the safety direction. It satisfies R6 by
  construction, being the body that decides compliance posture, and it is genuinely distinct from
  the wallet operators - more than default staffing achieves against R3.
- **Why the Council itself cannot be the committee:** a Gnosis Safe holds no private key (its
  `m`-of-`n` is Ethereum contract logic, which Bitcoin Script cannot evaluate); a plain multisig of
  its signers would re-break wall 3 on every owner rotation; and merging governance with the
  committee destroys governance escalation as the backstop for committee failure, because you
  cannot escalate to yourself.

## Current conclusions

1. **The cryptographic question is closed.** Every case-1 (no live party) construction fails R2 or
   R6/R7, and the only remaining design freedom is *which* live party and *how expensive*.
2. **A committee is not "the" answer in shape, but the shape is now forced.** Case 3 is a cost
   spectrum, and the one candidate that looked cheaper (Mechanism 6) turned out to be unbuildable
   against wall 3. The remaining freedom is staffing, where Council-appointed membership of a
   distinct threshold group is the cheapest answer that keeps every requirement.
3. **This is not a substitute for the compensation module, and vice versa.** Working the numbers
   in `proposal.md` §5.1 showed they address different harms: a stranding depositor is already
   close to whole in fungible tBTC (strandings are capped and small against the pool), so
   compensation's real job is **solvency repair owed to every tBTC holder**, while this family
   delivers the **specific in-kind guarantee** the product is actually selling. Skipping this
   family is only safe if depositors would accept fungible tBTC at par when custody fails - if the
   1-to-1 lineage promise is real, the complexity here is what makes it real.
4. **Recommendation: build compensation Tiers 0-1 first** (`../stranding-compensation-proposal.md`
   §10 - liability accounting and fee restitution, both cheap and useful regardless of what
   happens here). Tier 0 produces the stranding-frequency evidence that is the actual decisive
   input for whether this family is worth building at all, and at what cost tier.
5. **Committee failure is bounded, not solved - and the mechanism can cause the failure it
   prevents.** Even Mechanism 1 (deferred) gets stuck if its committee goes dark, and worse: resharing
   keeps the committee key alive only while the group keeps refreshing, so with no maximum custody
   term an anchor minted in 2027 can outlive several generations of committee. If the group ever
   falls below its threshold, that anchor's emergency branch is permanently unexercisable - the
   dead-key failure the mechanism exists to prevent, now caused by the mechanism. `proposal.md` §5
   lists redundancy (§5.1), compensation reuse, governance escalation, and a terminal disposition
   as the ways to bound this; none is built, and the last two are no longer optional.

## Files in this folder

- **`proposal.md`** - Mechanism 1, the retained design reference (deferred): problem statement,
  R1-R5, the mechanism itself, why it closes each requirement and where it doesn't, open
  questions, bounding committee failure, lifecycle interactions with the reservation state
  machine, and the core sequence-diagram flows.
- **`alternatives.md`** - the closure investigation: proves the design space is one dial (how long
  the claim stays illiquid), derives R6/R7, works through Mechanism 5 and its lien-fraction
  economics in full, surfaces and then withdraws Mechanism 6, and tabulates every other family
  that was considered and killed.
- **`addendum.md`** - the earlier-rejected Mechanisms 2-4, the revision history of how Mechanism 1
  itself evolved, the R1-R5 comparison table across all four original mechanisms, and secondary
  edge-case sequence diagrams (`Stranded` comparison, committee unavailable, disarm-and-recover).
- **`stranded.md`** - the existing status-quo fallback this whole folder competes with: plain
  explanation, a worked example, and the source-grounded mechanics of `notifyReservationStranded`.

## Open items

- Expected annual loss from wallet termination - unquantified, and it is what actually decides
  whether any mechanism in this folder is worth its cost (`alternatives.md` §6).
- **Capping the custody term.** No maximum exists today, which leaves the committee's key-liveness
  duty unbounded (`proposal.md` §5.1). This is needed by Mechanism 1 if ever built, not only by the
  rejected lien (`alternatives.md` §3.1).
- Committee composition, selection, and rotation (`proposal.md` §4); resolved in principle via
  threshold resharing (§5.1), not yet an operational plan.
- ~~Whether this family supersedes or coexists with `Stranded` for a given reservation
  (`proposal.md` §4 "Governance surface").~~ **Settled by the Decision: it does not supersede.
  `Stranded` stands as the fallback; the family is deferred.** Reopens only on the Decision's
  evidence triggers.
- Terminal disposition for a position stuck in `EmergencyExitArmed` with an unresponsive committee
  (`proposal.md` §5, item 4). The direction is now settled - governance force-burns the escrow and
  pays compensation, since force-refund would need proof the anchor is unspendable and a dead
  wallet cannot re-anchor - but it is unspecified, and it makes the compensation module a
  prerequisite rather than an alternative.

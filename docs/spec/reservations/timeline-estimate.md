# UTXO Reservations — Timeline Estimate (with testing hardening folded in)

Status: `[INFERENCE]` throughout — reasoned estimate, not a number the team
has published or committed to. Baseline assumes one engineer with heavy
agent leverage (see `feature-spec.md` §16 and the original
completion-estimate discussion for the pre-testing baseline). Testing scope
is `testing-plan.md`.

## 1. Baseline (before folding in testing recommendations)

| Phase | Estimate | Compresses with agents? |
|---|---|---|
| Engineering to code-complete (stack merge — **superseded, see §7: B is a rewrite, not a merge**, stranding-compensation module **Tiers 0-1**, fee-grinding fix, keep-core two-phase rework **incl. the two executor duties from the Stranded-decision review** — re-anchor-on-rotation and stranding-cleanup, integration tests, findings/reference docs, fork dry-run). The emergency-exit build is **out of scope** per the 2026-08-21 decision (§5). | ~33-49 engineer-days serialized -> ~4-6 weeks calendar with parallel agent workstreams | Yes |
| External audit engagement + fix/re-review cycle | ~4-8 weeks calendar | No — vendor calendar |
| Governance sign-off (2 unset params + launch approval) | ~1-3 weeks, overlaps with audit | No — DAO/Council calendar |
| **Total (baseline)** | **~10-14 weeks (2.5-3.5 months)** *(Engineering ~4-6 + Audit ~4-8 = ~8-14 alone; the extra ~2 weeks at the low end is a non-overlapping admin/handoff buffer, made explicit in §3 after the 2026-08-21 review — not a separate governance delay, since governance is stated to overlap audit)* | |

## 2. Testing-plan items, added effort, and whether they sit on the critical path

Three categories: items that must land **before** the audit submission
(add to the engineering phase, only partially compressible because some
depend on other work finishing first), items explicitly scoped to run
**during** the audit's calendar window (add ~0 critical-path time, since
that window already has idle engineer capacity beyond reactive fix work),
and a **testnet round** — a live-network validation phase that needs
code-complete before it can start, sits mostly on the critical path for
its setup + first runs, and sheds its tail into the audit window.

### Pre-audit (adds to the engineering phase)

| Item | Raw effort | Net add to critical path | Why net < raw |
|---|---|---|---|
| Foundry invariant/fuzz suite (4-6 handlers, tbtc-v2) | 3-5 days | ~2-3 days | Can run concurrently with stack review/merge (§3 of the merge plan) rather than after it. |
| `-race` in CI + fixing whatever it surfaces | 1-2 days | ~1 day | CI flag flip is free; fix time depends on what's found, budgeted small since no race is confirmed yet. |
| keep-core dispatch/coordination unit tests for reservations (`TestNode_HandleReservationProposal_*`, checklist inclusion) | 1-2 days | ~1 day | Tightly coupled to the already-budgeted 10-15 day coordination-executor rework — largely the same PR, small increment. |
| Extend fuzz-marshaling pattern to reservation proposals | <1 day | ~0 days | Two-line addition during the same wiring PR. |
| Gambit mutation-testing run + triage of weak assertions found | 2-3 days | ~2-3 days | Needs the test suite in a stable state first — genuinely sequential, can't overlap much. |
| Fork-based e2e lifecycle test (reserve->accept->settle->renew->partial-redeem->dissolve against a mainnet fork + Bitcoin regtest) | 4-6 days | ~3-4 days | Partially overlaps with the runbook's already-budgeted "fork dry-run of activation sequence" (2-3 days), but is larger in scope than that dry-run alone. |
| Multi-signer simulated integration test in keep-core (scale existing single-node harness to N=`GroupSize` signers, full lifecycle through real coordination rounds) | 5-7 days | ~4-5 days | Needs the coordination wiring done first, so it's mostly sequential after that lands; some overlap possible with fork e2e work above (different repo, different engineer-agent workstream). |
| TLA+ protocol model of the two-phase state machine | 3-5 days | ~1-2 days | Best done early/in parallel with implementation (it's meant to inform the design, not just check it after the fact) — largely absorbs into the existing engineering window if started on day one instead of at the end. |
| **Subtotal, pre-audit testing work** | **20-31 days raw** | **~14-19 days net** | |

### During-audit (parallel, ~0 added critical-path time)

| Item | Raw effort | Critical-path add |
|---|---|---|
| Narrow Certora CVL specs (4 invariants: claim≡anchor, storage append-only, supply conservation, no double-settle) | 7-10 days | ~0 — fits inside the 4-8 week audit calendar window alongside reactive fix work |
| Property-based testing for keep-core reservation state parsing (`rapid`) | 2-3 days | ~0 — same window |
| **Subtotal, audit-parallel** | **9-13 days raw** | **~0 days** |

### Testnet round (staging — live network + Bitcoin testnet)

A distinct phase, not just another test-tooling item: it exercises the
full stack with real multi-operator coordination, a live L1, and real
Bitcoin-testnet UTXOs, and it is the validation ground for the two
executor duties added in the Stranded-decision review (§5) — re-anchor-on-
rotation and stranding-cleanup — plus the whole liveness-downtime -> `Stranded`
path our analysis rests on. Setup and first runs sit on the critical path
(before audit submission); the tail (extended runs, soak, live-only bug
fixes) overlaps the audit window's idle capacity.

**Prerequisite note — accelerated parameters.** The protocol's timers are
governable but default long (term 90-730 days, renewal window 30 days,
dissolution delay 7 days, action/moving-funds timeouts). A meaningful
testnet round cannot wait out real terms, so governance must set
accelerated testnet parameters. That is itself a small engineering task
(param plumbing + a governance config used only on the testnet deployment)
and changes which behaviors are faithfully observed — rotation,
re-anchor, and stranding scenarios must be force-driven via rotation
requests and deliberate downtime rather than observed by waiting. Budget
it explicitly below.

| Item | Raw effort | Net add to critical path | Why net < raw |
|---|---|---|---|
| Staging environment: testnet contract deployment, accelerated-param governance config, operator(key set) bring-up, BTC-testnet funding/UTXOs, monitoring wiring | 3-5 days | ~3-4 days | Deployment is scriptable and parallelizable; the accelerated-param config and operator lifecycle are the sequential core. |
| Core lifecycle runs on live network (accept -> whole + partial redeem -> renew -> dissolve -> re-anchor, real coordination rounds) | 4-6 days | ~3-4 days | Wall-clock waits between accelerated events leave the engineer free to run parallel workstreams; raw time is dominated by run duration, not attention. |
| **Liveness/stranding drill** (forced Scenario: take an operator set dark -> MovingFunds -> executor re-anchor attempt -> movingFundsTimeout -> Terminated -> `notifyReservationStranded`; verify capacity release, event, monitoring, and the Tier 0 liability accounting that consumes it) | 2-3 days | ~2 days | Sequenced after the core runs and the coordination-executor rework it validates; deliberately orchestrated, so bounded. |
| **Subtotal, testnet pre-audit core** | **9-14 days raw** | **~8-10 days net (~+2 weeks critical path)** | |
| Extended runs / soak + triage/fix/rerun of bugs that only appear on a live network with real Bitcoin (real fee markets, real coordination, real downtime) | 3-5 days raw | ~0 net | Absorbs into the audit window's idle capacity alongside the other during-audit items; running longer is free of critical-path cost. |
| **Subtotal, testnet audit-overlap tail** | **3-5 days raw** | **~0 days** | |

## 3. Net effect on the timeline

- Pre-audit engineering phase: **~4-6 weeks -> ~7-10 weeks** (add ~14-19
  engineer-days, roughly **+3 to +4 weeks**).
- **Testnet round, pre-audit core: +~8-10 engineer-days (~+2 weeks)** on
  top of engineering (staging, accelerated-param config, core runs, the
  liveness/stranding drill). Its audit-overlap tail adds ~0 critical-path
  time.
- Audit + governance phase: unchanged at ~4-8 weeks calendar — the Tier 3
  items (Certora, property-based Go tests) *and* the testnet tail absorb
  into this window's idle capacity rather than extending it.

| | Baseline | With testing folded in | + Testnet round |
|---|---|---|---|
| Engineering to code-complete + hardened | ~4-6 weeks | ~7-10 weeks | ~9-12 weeks |
| Audit + fixes + governance | ~4-8 weeks | ~4-8 weeks (unchanged) | ~4-8 weeks (unchanged) |
| Non-overlapping admin/handoff buffer *(undocumented until this review; applies uniformly below — see §1)* | ~2 weeks | ~2 weeks | ~2 weeks |
| **Total elapsed** | **~10-14 weeks** | **~13-18 weeks (~3.25-4.5 months)** | **~15-20 weeks (~3.75-5 months)** |

## 4. Reading this estimate

- The +3-4 week addition buys real reduction in audit risk: a fuzzed
  invariant suite, a mutation-tested Mocha suite, a fork-verified e2e
  lifecycle, a multi-signer keep-core simulation, and a model-checked
  protocol design, all in place *before* the auditor's clock starts. Any
  of these that would otherwise surface as an audit finding costs more
  than 3-4 weeks in finding-fix-re-review cycles once the auditor is
  already engaged and being paid by the hour/week.
- The single biggest lever to avoid stretching the total further: do the
  TLA+ protocol model **first**, before finishing the keep-core two-phase
  wiring, not after. If it surfaces a real protocol-level issue, catching
  it before wiring code around the current design is far cheaper than
  catching it during the multi-signer simulation (item 7) or, worse,
  during audit.
- **The testnet round is where the Stranded-executor duties and the whole
  liveness-downtime -> `Stranded` path actually get exercised.** The two
  executor duties added in the decision review (§5) — re-anchor-on-rotation
  and stranding-cleanup — and the operator-goes-dark -> `MovingFunds` ->
  timeout -> `Terminated` -> `Stranded` behavior the analysis rests on cannot
  be proven by unit/fork/simulation tests alone; they need a live L1 with
  real multi-operator coordination and real Bitcoin testnet. Plan for the
  accelerated-parameter prerequisite (the native timers are governable but
  default to 90+ day terms, so testnet must shorten them) and drive rotation
  and downtime scenarios deliberately rather than waiting them out.
- This estimate does not include time for whatever the audit itself
  finds beyond the fix/re-review buffer already in the baseline — a
  finding that requires redesigning a piece of the two-phase state
  machine (rather than a local fix) would extend the timeline further and
  isn't something pre-audit testing can fully rule out.

## 5. Changes from the 2026-08-21 "Stranded" decision review

This estimate was revisited after two things: the decision to lock on the
existing `Stranded` fallback for terminated-wallet reservations (and not
build the emergency-exit mechanism), and the stranded-design analysis the
lock produced (`exit/stranded.md`). Net timeline impact is
**~nil to marginally favorable**; the deltas:

- **Emergency-exit build removed from scope.** The decision closes the
  `exit/` family as a *retained design reference* — not built
  (Decision block in `exit/README.md`). No exit-scoped
  engineering appears in, or is added to, this timeline. The only
  loss-story build that remains is the compensation module, below.
- **Compensation module = Tiers 0-1, and it is now the priority loss-story
  deliverable.** Already in the baseline engineering phase; the decision
  sharpens what it must be. Tier 0 (cumulative stranded-liability
  accounting) is not just compensation plumbing — it is the
  **evidence-gathering instrument** the decision hinges on: it produces
  the stranding-frequency × anchor-value number that would, if it ever
  exceeds mechanism cost, reopen the exit question. Build Tier 0 first; it
  feeds the re-open decision loop rather than being a leaf deliverable.
- **Two executor duties added to the keep-core rework, absorbed by its
  existing budget.** The spec's §13 executor section now requires (a)
  re-anchor every open reservation on `Live -> MovingFunds`, and (b) call
  `notifyReservationStranded` for terminated wallets holding un-stranded
  reservations. These are small behavioral additions inside the already-
  budgeted 10-15 day coordination-executor rework (same PRs, same wiring),
  so they add at most a rounding day — no new phase, no critical-path
  change.
- **H-06 documentation sharpened, no timeline effect.** The spec now states
  explicitly that stranding never changes a depositor's tBTC balance, and
  that `Terminated` may be a liveness failure rather than theft (three
  causes, two non-malicious). This is docs-only — it clarifies monitoring
  and compensation expectations but adds no implementation work.
- **Liveness-dominant frequency (evidence-shaping, not schedule-shaping).**
  Expected stranding is dominated by operator downtime, not fraud
  (`exit/stranded.md` §3.4). This does not move any date; it confirms Tier 0's
  importance (it must measure real liveness-downtime strandings) and is
  the input that would justify revisiting scope later.
- **A testnet round was added (§2, §3) and is the natural validator for the
  material above.** It is where the two new executor duties and the
  liveness-downtime -> `Stranded` path get proven against a live network,
  and where the Tier 0 evidence-gathering actually starts producing real
  stranding-frequency data. It adds ~+2 weeks to engineering (pre-audit
  core) with its tail absorbed by the audit window — see §2's testnet
  subsection and the recomputed §3 totals.

**Bottom line:** the decision itself stays ~nil-to-favorable — it removes
one line of open-ended future work, folds two small executor duties into
budget already reserved for them, converts the compensation module into
the clearly-prioritized Tier 0-first deliverable, and sharpens the spec
without adding schedule. The **testnet round** is a separate, deliberate
addition on top (validation ground for those executor duties and the
liveness->`Stranded` path), and it is what moves the §3 totals: ~13-18
weeks -> **~15-20 weeks**. Revisit both if Tier 0's real stranding-
frequency data later justifies reopening the exit decision.

## 6. Effect of the create-only milestone split (2026-08-21)

`roadmap.md` splits delivery into a create-only m1 and a later m2. The
numbers above are **unchanged** by that split, for reasons worth stating
because the intuitive expectation is a reduction:

- **The Solidity surface does not shrink.** Only `#1096` is deferred; the
  rest of the stack ships, because `#1092`'s expiry model is structural to
  `#1093`+ rather than additive (`roadmap.md` §0.1). Create-only comes from
  an m1 vault that exposes no redemption or renewal entry point, not from
  omitting PRs. So the audited contract surface is 7 of 8 PRs either way.
  **Reversed by the B decision — see §7.** B is a rewrite, so the m1 surface
  drops to 5,261 lines (-43%), and the vault must *ship* its entry points
  behind flags rather than omit them (`roadmap.md` §0.7).
- **The keep-core rework does not shrink either.** Dissolution cannot be
  deferred: it is permissionless, so an unwired dissolution path lets anyone
  slash honest wallets on timeout (`roadmap.md` §0.6). m1 therefore needs
  acceptance, re-anchor **and** dissolution — only the redemption proposal
  drops out, which is the smallest of the four.
  **Also reversed — see §7.** B cuts dissolution from the contracts entirely,
  so §0.6's vector does not exist in m1 and keep-core needs acceptance and
  re-anchor only. This is B's one genuine saving, ~300-500 production Go.
- **What the split does buy is a later deadline, not less work.** The
  in-kind redemption promise moves from "before launch" to "before the
  earliest `dissolutionEligibleAt`", roughly 12 months out under §1.4's
  term. That converts a launch blocker into a dated commitment, which is a
  scheduling win rather than an effort saving.
- **One risk the split adds:** the m1 vault must ship redemption behind a
  flag, because `ReservationVault` is not upgradeable and re-pointing it is
  blocked until every position closes (`roadmap.md` §2.2). That is a small
  addition to the m1 vault's own audit surface, not a new phase.

**Net: no change to §3's ~15-20 weeks.** The split changes *when* in-kind
redemption must exist, not how much must be built and audited before launch.

## 7. Effect of the variant B decision (2026-08-21)

The milestone 1 plan was officially changed on 2026-08-21 from the stacked whole-PR path (eight PRs merged sequentially) to variant B: an essentials-only rewrite of m1. This document's baseline (§1) and prior revisions (§5-§6) still describe the stacked plan; this section records the delta introduced by the B decision, following the doc's convention of retaining superseded analysis and marking it as such.

- **m1 engineering effort**: shifted from "review + merge + rework" of the stacked PRs to writing 5,261 production Solidity lines fresh, extracting logic from the already-reviewed PRs. Extraction from reviewed code is cheaper per line than greenfield but more expensive than a simple merge; net direction uncertain without precise calibration [direction only].  
- **Audit shape**: the stacked plan concentrated ~9,206 production lines in m1 with a near-zero m2 delta (13.2:1 ratio), implying one large audit engagement plus trivial m2 cleanup. Variant B splits the audited surface into two comparable chunks: 5,261 lines in m1 and ~4,641 lines in m2 (1.13:1 ratio). Vendor audit calendar windows do not compress, so this change replaces one large audit with two sequential audits of similar magnitude, increasing calendar duration. The effect is dominant and pushes total program duration up despite m1's smaller absolute size.  
- **keep-core engineering**: m1 B requires ~1,100-1,400 production Go lines (same rough scale as the stacked plan's m1 keep-core rework). m2 after B must add two action types (Redemption and Dissolution) rather than one; the keep-core two-phase client remains the critical path item, so m2 effort increases critical-path load [direction only].  
- **New m1 launch gates**: scope absent from both earlier plans includes (a) a global active-position cap enforced at acceptance, (b) the m1 vault shipping its complete entry-point surface behind pause flags, (c) `reservationsByAnchorUtxo` reconciliation, and (d) writing `dissolutionEligibleAt` at acceptance despite having no reader. Each is small individually but additive; net direction up [direction only].  

No precise day counts are assigned above where inputs only support a direction; where the contract provides line counts or ratios, those figures are cited directly.

# UTXO Reservations - Specification and design records

Start with the [canonical verification specification](verification/README.md).
It covers the current M1 contract, SDK and keep-core design at explicit evidence pins.
It separates intended rules, implementation observations and test/rehearsal evidence.
It does not claim runtime verification from a source citation.

The earlier documents remain as design history and operational references.
The verification set records the contradictions it found with the current design, and it supersedes the earlier documents where the two disagree.

## Decision log

The table retains the historical scope decisions.
The verification index records later approved additions and evidence snapshots.
If documents conflict, record the conflict and its deciding evidence; do not combine incompatible designs.

| Date | Decision | Authoritative record |
|---|---|---|
| 2026-08-21 | The existing `Stranded` fallback is the accepted outcome for a terminated wallet's reservations; the emergency-exit mechanism is **not built**, retained only as design reference. | `exit/README.md` |
| 2026-08-21 | Milestone 1 is **variant B with a minimal router** — create, custody and re-anchor only, no dissolution, built as an essentials-only rewrite rather than by merging the eight-PR stack. | `roadmap.md` §1 |
| 2026-09-07 | `#1122` reverted `#1121`: no reserved-redemption / veto / renewal surface in the M1 Bridge. | `requirements.md` §3; `roadmap.md` §1.2 |
| 2026-09-24 | **Option B.** The M1 `ReservationVault` ships minimal (as landed in `4d549e64`); redemption and renewal are delivered in m2 via a **new vault deployment and depositor migration ceremony**, not via unpause flags on the M1 vault. **Confirmed by the project owner 2026-09-28.** | `m1-b-implementation.md` §3 |
| 2026-09-28 | **Fee-reserve floor (D-3).** `sweepFees` repays `inKindFeeDebtSat` from the vault's full current balance before comparing against `feeReserveTarget`; the target constrains only the sweepable surplus, not debt repayment. Recorded as an M1 decision; no contract change. | `m1-b-implementation.md` §3, §5; `requirements.md` §12 |
| 2026-09-28 | **Late-settlement fallback (E-2).** If governance revokes the reveal-time vault's trust before a late acceptance proof settles, the Bridge credits the depositor directly via `Bank.increaseBalances` with no initiation fee. Settlement is never forced back through a revoked vault. | `m1-b-implementation.md` §3, §5; tbtc-v2 `solidity/docs/RESERVATION_CAPS_DEPLOYMENT.md` (Late-Settlement Fee Fallback When Vault Trust Is Revoked (E-2)) |
| 2026-09-28 | **Client activation ordering (C-4).** Release and deploy a keep-core build whose `reservationsActivationBlocks` contains the target network's entry **before** `setVaultStatus(vault, true)` executes; the on-chain activation transaction must be scheduled at or after that block, otherwise reserved reveals settle on-chain while no client ever schedules acceptance or re-anchor. | tbtc-v2 `solidity/docs/RESERVATION_CAPS_DEPLOYMENT.md` ("Client Activation Ordering Gate (C-4)"); `m1-b-implementation.md` §5 |

M1 implementation and deployment are separate milestones.
The reservations branches carry reviewed changes ahead of the epic pull requests.
The verification set names the exact revisions it inspected.
The live cross-PR tracker is `docs/plans/reservations-epic-tracker.md` on `reservations-epic`.
Deployment readiness remains subject to the release runbook and its operator/key-holder gates.

`m1-variant-comparison.md` preserves the A+/B reasoning. It is not the current implementation specification.

## Revision history

The table is the changelog of the set: it records what each revision added, changed or removed.
A change that alters a claim appends a row and names the text a reader of an earlier revision must re-read.
Entries carry the date the revision entered this repository.

| Date | Change |
|---|---|
| 2026-10-29 | Added the UTXO reservations design and verification specification, with pinned citations and a derived traceability matrix |

---

## File index (by role)

### Specification (M1)
| File | Role |
|---|---|
| `requirements.md` | Earlier M1 requirements and recorded design intent. Use `verification/` for current invariants, stories, parameter rules and implementation evidence. |
| `architecture.md` | Earlier M1 architecture with subsequent additions. The canonical model is `verification/01-system-model.md`. |
| `handoff-capacity.md` | D31 hand-off design: the acceptance reserve gate `P + 1 <= max(0, L - R) * N` and the privileged hand-off from a non-Live source. Implemented at `tbtc-v2@806b1106f` and `keep-core@d40f6aadc` (reservations branches, not deployed); see `verification/13-approved-design-deltas.md`. |
| `releases.md` | **Release assignment.** Which goals, requirements, stories, accepted risks and gates belong to M1 and which to M2, by pointer to the document that owns each one. It states the two product milestones against the epic tracker's M0 to M7 delivery phases, records the two places where `requirements.md` is stale against the published code, and lists the M2 gaps as open questions. It restates no requirement; the cited text stays authoritative. |

### Spec & evidence
| File | Role |
|---|---|
| `feature-spec.md` | Reverse-engineered spec of the **full** reservation feature as the original nine-PR stack implements it — the **m2 target**, not what m1 ships: two-phase settlement machine, data model, caps/fees, wallet-lifecycle integration, governance surface, deployment runbook, review-findings table, cross-cutting invariants, consolidated open questions (§15), gap analysis (§16), FROST interaction (§17). Carries a "Status vs M1 code (2026-09-28)" banner flagging where the reverse-engineered stack diverges from the M1 code actually implemented (`reservations-upgrade` @ `9f8f5ef1` / `reservations-epic` @ `f66f11240`). |
| `inventory/` | **Evidence tier, pinned to the pre-rewrite eight-PR stack** (verified against `feat/utxo-reservation-guards`, the `#1094` tip — see `inventory/README.md`'s own provenance note). Seven line-cited source-verification fragments (`data-model`, `proofs`, `router`, `vault`, `touchpoints`, `pr-map`, `keep-core`) behind `milestone-inventory.md`. Each fragment carries a "Status vs M1 code (2026-09-28)" banner where its pre-rewrite citations diverge from the code the M1 rewrite actually implements. See `inventory/README.md` for its own index, the `#1102` provenance caveat, and the `PD-N` decision-ID namespace. |
| `pr-review-followups.md` | Commit-pinned review artifact (multi-lens reviews of #1088/#1102). Follow-up items that need a design decision or a new mechanism, each ending in a "verify, don't assume" action aimed at a later PR. Evidence log that `feature-spec.md` §15/§16 point to. |

### Planning
| File | Role |
|---|---|
| `roadmap.md` | **Scope decision.** Milestone decomposition for a create-only first release: §0 the source-verified facts it turns on (§0.7 = the two-layer upgradeability rule), §1 the **decided m1 = variant B** scope, what defers to m2, PR edits and implementation gaps. §0 also records the earlier decisions it reversed. |
| `m1-variant-comparison.md` | **The argument that was weighed.** Side-by-side of A+ and B: shared feature set, the single difference (dissolution), measured line counts, EIP-170 arithmetic by subtraction, and §5.3's verified B endgame — saturation leads to seized operator stake and stranded depositors. §5.4/§5.5 are the per-variant hole lists. §6 records that B was chosen and retains the A+ recommendation unchanged as the risk register that comes with it. |
| `m1-b-implementation.md` | **Build scope for the decided variant.** Router surface (§2), the vault's minimal surface — redemption and renewal deferred to m2 via a new vault deployment and depositor migration ceremony, per the 2026-09-24 Option B decision (§3) — launch gates (§4; §4.2 superseded), operational duties (§5), and what m2 must then build (§6). |
| `timeline-estimate.md` | Schedule: phases, baseline + testing fold-in, testnet round (added 2026-08-21), and the §5 rewrite after the Stranded-decision review. §1's baseline and §§5-6 still price the stacked plan; **§7 is the variant B delta**. |
| `testing-plan.md` | Test & hardening plan: pre-audit vs during-audit tooling (Foundry invariants, TLA+, multi-signer sim, fork e2e, Certora, etc.), effort, critical-path impact. **Superseded in part**: read §4 before executing §3. |
| `milestone-inventory.md` | **The completeness ledger.** Every item the m1 rewrite must ship, declare or build new, with source, PR attribution and milestone assignment. Four trap sections catch what a straightforward extraction loses: items with no extraction source, fields carried for layout only, writes that look dead but are load-bearing, and wrong claims in the existing docs (C-1 to C-8). §2.10 covers the non-code obligations. Ends in a numbered open-decisions register (D-1 to D-27) other docs cite; 16 blocking, 11 deferrable. Its line-cited evidence is `inventory/`. |
| `pr-strategy.md` | **How m1 actually ships.** Assesses five options for delivering a rewrite while preserving the eight existing PRs as reference, answers what makes a PR permanently readable (measured, not asserted — §3), and recommends a per-repo PR decomposition with branch names and review focus. |
| `epic-merge-plan.md` | Reference record of the 8-PR tbtc-v2 stack plus standalone keep-core #4238: the verified PR inventory, per-PR extraction guidance, and the `gh-stack` mechanics. No longer a delivery plan (superseded 2026-08-21); `pr-strategy.md` is the live one. |

### Delivery & readiness (M1)
| File | Role |
|---|---|
| `m1-tbtc-v2-readiness/` | Solidity/Bridge-side M1 readiness audit: gap analysis against the landed code (`01-gap-analysis.md`), user stories for the M1-reachable paths (`02-user-stories.md`), and a follow-up PR spec for the two gaps found originally (`03-followup-pr-spec.md`). Originally audited against an earlier `reservations-upgrade` tip (`8c5a2f4d`, before the Option B vault decision); **re-verified 2026-09-28 against `9f8f5ef1`** (the current M1 code pin) — the re-anchor dissolution-gate gap was fixed and merged (tbtc-v2 `#1120`, 2026-09-03), and the original vault gap (missing paused redemption entry points) is superseded by the 2026-09-24 Option B decision — the M1 vault ships minimal instead, see `m1-b-implementation.md` §3. |
| `m1-keep-core-readiness/` | Go-client-side M1 readiness audit: gap analysis (`01-gap-analysis.md`), user stories for acceptance/re-anchor/timeout/stranding (`02-user-stories.md`), a delta-changes task list (`03-delta-changes.md`), and the sequenced implementation plan that closed them (`04-implementation-plan.md`) across keep-core PRs #4276-#4280 (all merged 2026-09-03). |
| `../../plans/reservations-epic-tracker.md` | **Live milestone tracker.** Every milestone and open item across the reservation PRs: the M0 to M7 delivery phases of M1, the drill status, and the related out-of-scope list (M2 reserved redemption, multi-term). It replaces the status sections of the earlier delivery log from its recorded start date; that log is not in this tree. |

### Loss-story design
| File | Role |
|---|---|
| `stranding-compensation-proposal.md` | Compensation module design (Tiers 0-1). The **only buildable** loss-story piece under the Decision; Tier 0 doubles as the stranding-frequency evidence instrument that could reopen the exit question. |
| `shortfall-design-space.md` | Who-pays analysis when a wallet dies holding anchors (Spaces A/B/C). Rejects Space A (slashing invariance) and Space B (fungibility); finds Space C (mint < lock) only viable **conditional on an unbuilt `anchorAmount`/`mintedAmount` decoupling** — **not adopted, not scoped** (LTV value and the §4.3 assessment are still open). |
| `exit/stranded.md` | **LIVE.** The `Stranded` fallback: preconditions, the three causes of `Terminated`, a worked example. m1's **only** terminal path, so this is not optional reading despite its folder. |
| `exit/` (rest) | Emergency-exit design family — **deferred, retained as reference** (see `exit/README.md` for its own index and the Decision block). |

### Cross-cutting
| File | Role |
|---|---|
| `frost-reservations-interaction.md` | Interaction with the separate FROST/Schnorr migration: no forced sequencing, re-anchor as the migration path, one pending settlement-side patch, storage-merge parity risk. |

### Public docs
| File | Role |
|---|---|
| `vba-technical-diagram.md` | **Publishable GitBook page** replacing docs.threshold.network "Verifiable Bitcoin Accounts > Technical Diagram", whose covenant/PSBT design never shipped. Describes milestone 1 only, with a v2 disclaimer; written for institutional readers, so it carries no internal IDs, file paths or placeholder numbers. Claims checked against tbtc-v2 `reservations-upgrade` @ `9f8f5ef1` and keep-core `reservations-epic` @ `f66f11240` (re-pinned 2026-09-28 from the stale `4390a7998`; the page is being re-verified against these refs in parallel — re-check if either moves). |

---

## Reading order
Current review order:

1. [Verification index and evidence snapshots](verification/README.md).
2. System model, invariants and parameter/governance rules.
3. User stories, edge cases and failure modes.
4. Recovery, threat model and accepted risks.
5. Cross-repo contract, traceability matrix and finding adjudication.
6. The canonical verification set before using older material.

The order below is the historical reading guide, not the current review checklist.


Scope first. The single most common wrong turn is reading `feature-spec.md`
front-to-back and concluding that milestone 1 builds the whole feature; it
describes the **full** feature, which is the m2 target.

1. This file — the decision log above.
2. `requirements.md` — the M1 requirements: goals, non-goals, scope, user
   stories, functional requirements, open decisions.
3. `architecture.md` — the M1 architecture: components, flows, state
   machines, storage and deployment.
4. `roadmap.md` §1 — what milestone 1 is. §0.7 is the upgradeability rule it
   turns on; §0.8 is the wallet-lifecycle finding that bounds it.
5. `m1-b-implementation.md` — what milestone 1 *builds*: router surface, the
   vault's minimal surface, launch gates (§4; §4.2 superseded), operational
   duties.
6. `milestone-inventory.md` §1.2 and §7 — the completeness check, and the
   D-1..D-27 decision register, including resolution history and the remaining
   deferrable items. `inventory/` holds the line-cited evidence behind every row.
7. `feature-spec.md` — the full feature, i.e. the m2 target (start §1-§4, skim
   the rest). Read it knowing §5 renewal, §4's redemption paths and dissolution
   are all m2.
8. `exit/README.md` then `exit/stranded.md` — the Decision and why `Stranded`
   won, then the mechanics of m1's only terminal path. Before building anything.
9. `shortfall-design-space.md` -> `stranding-compensation-proposal.md` — the
   loss story, in that order (the second's Space A framing is rejected by the
   first).
10. `pr-strategy.md` — how the work becomes pull requests, with
    `epic-merge-plan.md` as the superseded stack record behind it.
11. `testing-plan.md` (read its §4 first) -> `timeline-estimate.md` (§7 is the
    variant B delta) — hardening and schedule.
12. `frost-reservations-interaction.md` — cross-cutting; `pr-review-followups.md`
    with `feature-spec.md` §15/§16 — what is still open.

`m1-variant-comparison.md` is not in the order: the A+/B choice is **closed**
(B, 2026-08-21). Read it for the argument that was weighed and the risk register
B carries (§5.4, §6).

**Note on duplicated-looking references:** several docs summarize a conclusion
that another doc carries in full (e.g. `feature-spec.md` §17 summarizes
`frost-reservations-interaction.md`; `feature-spec.md` §15 points at
`pr-review-followups.md`). That is intentional progressive disclosure: the
summary is where you notice the conclusion, the deeper doc is where you check it.

Note that the entry point is **this file**, not `feature-spec.md`. That changed
on 2026-08-21: `feature-spec.md` is the canonical description of the *feature*,
but it describes the full feature, so entering there leads a reader to believe
milestone 1 builds all of it. Scope lives here and in `roadmap.md` §1.

*Draft, 2026-09-28. Kept under `docs/spec/reservations/`; promoted into version control 2026-08-21.*

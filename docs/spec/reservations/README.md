# UTXO Reservations — Design & Planning Documents

Working documents for the tBTC v2 UTXO-reservation feature (tbtc-v2 PR chain
#1088, #1090-#1096; keep-core #4238). Reverse-engineered from the open/draft
PR stack, not an authoritative protocol spec that the team has published —
parameter values, findings and schedules here are provisional until
governance sign-off and the external audit (see `feature-spec.md` §10, §15).

**Decision anchor (2026-08-21):** the existing `Stranded` fallback is the
accepted outcome for a terminated wallet's reservations; the emergency-exit
mechanism is **not built** and retained only as design reference. The
authoritative Decision block (including the re-open triggers) lives in
`exit/README.md`. Every doc in this folder is subordinate to that decision.

**Scope decision (2026-08-21):** milestone 1 is **variant B with a minimal
router** — create, custody and re-anchor only, no dissolution, built as an
essentials-only rewrite rather than by merging the eight-PR stack.
`m1-b-implementation.md` is the buildable form; `roadmap.md` §1 is the
authoritative scope statement and §0.7 the upgradeability rule it turns on.
`m1-variant-comparison.md` recommended A+ and is retained unchanged as the
argument that was weighed plus the risk register the choice carries.

---

## File index (by role)

### Spec & evidence
| File | Role |
|---|---|
| `feature-spec.md` | **Canonical description of the feature** (not the entry point — this file is; see the reading order). Reverse-engineered spec of the reservation feature as the PR stack actually implements it: two-phase settlement machine, data model, caps/fees, wallet-lifecycle integration, governance surface, deployment runbook, review-findings table, cross-cutting invariants, consolidated open questions (§15), gap analysis (§16), FROST interaction (§17). |
| `inventory/` | **Evidence tier.** Seven line-cited source-verification fragments (`data-model`, `proofs`, `router`, `vault`, `touchpoints`, `pr-map`, `keep-core`) behind `milestone-inventory.md`. See `inventory/README.md` for its own index, the `#1102` provenance caveat, and the `PD-N` decision-ID namespace. |
| `pr-review-followups.md` | Commit-pinned review artifact (multi-lens reviews of #1088/#1102). Follow-up items that need a design decision or a new mechanism, each ending in a "verify, don't assume" action aimed at a later PR. Evidence log that `feature-spec.md` §15/§16 point to. |

### Planning
| File | Role |
|---|---|
| `roadmap.md` | **Scope decision.** Milestone decomposition for a create-only first release: §0 the source-verified facts it turns on (§0.7 = the two-layer upgradeability rule), §1 the **decided m1 = variant B** scope, what defers to m2, PR edits and implementation gaps. §0 also records the earlier decisions it reversed. |
| `m1-variant-comparison.md` | **The argument that was weighed.** Side-by-side of A+ and B: shared feature set, the single difference (dissolution), measured line counts, EIP-170 arithmetic by subtraction, and §5.3's verified B endgame — saturation leads to seized operator stake and stranded depositors. §5.4/§5.5 are the per-variant hole lists. §6 records that B was chosen and retains the A+ recommendation unchanged as the risk register that comes with it. |
| `m1-b-implementation.md` | **Build scope for the decided variant.** Minimal router surface (20 of 24 entry points, with what was cut and why), the vault's full-surface requirement and the initiation-only pause rule, the **five** launch gates (§4.1-§4.5), operational duties, and what m2 must then build. |
| `timeline-estimate.md` | Schedule: phases, baseline + testing fold-in, testnet round (added 2026-08-21), and the §5 rewrite after the Stranded-decision review. §1's baseline and §§5-6 still price the stacked plan; **§7 is the variant B delta**. |
| `testing-plan.md` | Test & hardening plan: pre-audit vs during-audit tooling (Foundry invariants, TLA+, multi-signer sim, fork e2e, Certora, etc.), effort, critical-path impact. **Superseded in part**: read §4 before executing §3. |
| `milestone-inventory.md` | **The completeness ledger.** Every item the m1 rewrite must ship, declare or build new, with source, PR attribution and milestone assignment. Four trap sections catch what a straightforward extraction loses: items with no extraction source, fields carried for layout only, writes that look dead but are load-bearing, and wrong claims in the existing docs (C-1 to C-8). §2.10 covers the non-code obligations. Ends in a numbered open-decisions register (D-1 to D-27) other docs cite; 16 blocking, 11 deferrable. Its line-cited evidence is `inventory/`. |
| `pr-strategy.md` | **How m1 actually ships.** Assesses five options for delivering a rewrite while preserving the eight existing PRs as reference, answers what makes a PR permanently readable (measured, not asserted — §3), and recommends a per-repo PR decomposition with branch names and review focus. |
| `epic-merge-plan.md` | Reference record of the 8-PR tbtc-v2 stack plus standalone keep-core #4238: the verified PR inventory, per-PR extraction guidance, and the `gh-stack` mechanics. No longer a delivery plan (superseded 2026-08-21); `pr-strategy.md` is the live one. |

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

---

## Reading order

Scope first. The single most common wrong turn is reading `feature-spec.md`
front-to-back and concluding that milestone 1 builds the whole feature; it
describes the **full** feature, which is the m2 target.

1. This file — the two decisions above.
2. `roadmap.md` §1 — what milestone 1 is. §0.7 is the upgradeability rule it
   turns on; §0.8 is the wallet-lifecycle finding that bounds it.
3. `m1-b-implementation.md` — what milestone 1 *builds*: router surface, the
   vault's full-surface requirement, the five launch gates, operational duties.
4. `milestone-inventory.md` §1.2 and §7 — the completeness check and the
   D-1..D-27 open decisions that gate building. `inventory/` holds the
   line-cited evidence behind every row.
5. `feature-spec.md` — the full feature, i.e. the m2 target (start §1-§4, skim
   the rest). Read it knowing §5 renewal, §4's redemption paths and dissolution
   are all m2.
6. `exit/README.md` then `exit/stranded.md` — the Decision and why `Stranded`
   won, then the mechanics of m1's only terminal path. Before building anything.
7. `shortfall-design-space.md` -> `stranding-compensation-proposal.md` — the
   loss story, in that order (the second's Space A framing is rejected by the
   first).
8. `pr-strategy.md` — how the work becomes pull requests, with
   `epic-merge-plan.md` as the superseded stack record behind it.
9. `testing-plan.md` (read its §4 first) -> `timeline-estimate.md` (§7 is the
   variant B delta) — hardening and schedule.
10. `frost-reservations-interaction.md` — cross-cutting; `pr-review-followups.md`
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

*Draft, 2026-08-21. Kept under `docs/spec/reservations/` — formerly the
`agent-docs/` scratchpad; promoted into version control 2026-08-21.*
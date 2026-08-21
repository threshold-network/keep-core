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
| `feature-spec.md` | **Canonical entry.** Reverse-engineered spec of the reservation feature as the PR stack actually implements it: two-phase settlement machine, data model, caps/fees, wallet-lifecycle integration, governance surface, deployment runbook, review-findings table, cross-cutting invariants, consolidated open questions (§15), gap analysis (§16), FROST interaction (§17). |
| `pr-review-followups.md` | Commit-pinned review artifact (multi-lens reviews of #1088/#1102). Follow-up items that need a design decision or a new mechanism, each ending in a "verify, don't assume" action aimed at a later PR. Evidence log that `feature-spec.md` §15/§16 point to. |

### Planning
| File | Role |
|---|---|
| `roadmap.md` | **Scope decision.** Milestone decomposition for a create-only first release: §0 the source-verified facts it turns on (§0.7 = the two-layer upgradeability rule), §1 the **decided m1 = variant B** scope, what defers to m2, PR edits and implementation gaps. §0 also records the earlier decisions it reversed. |
| `m1-variant-comparison.md` | **The argument that was weighed.** Side-by-side of A+ and B: shared feature set, the single difference (dissolution), measured line counts, EIP-170 arithmetic by subtraction, and §5.3's verified B endgame — saturation leads to seized operator stake and stranded depositors. §5.4/§5.5 are the per-variant hole lists. §6 records that B was chosen and retains the A+ recommendation unchanged as the risk register that comes with it. |
| `m1-b-implementation.md` | **Build scope for the decided variant.** Minimal router surface (20 of 24 entry points, with what was cut and why), the vault's full-surface requirement and the initiation-only pause rule, the four launch gates, operational duties, and what m2 must then build. |
| `timeline-estimate.md` | Schedule: phases, baseline + testing fold-in, testnet round (added 2026-08-21), and the §5 rewrite after the Stranded-decision review. |
| `testing-plan.md` | Test & hardening plan: pre-audit vs during-audit tooling (Foundry invariants, TLA+, multi-signer sim, fork e2e, Certora, etc.), effort, critical-path impact. |
| `milestone-inventory.md` | **The completeness ledger.** Every item the m1 rewrite must ship, declare or build new, with source, PR attribution and milestone assignment. Four trap sections catch what a straightforward extraction loses: items with no extraction source, fields carried for layout only, writes that look dead but are load-bearing, and wrong claims in the existing docs (C-1 to C-8). §2.10 covers the non-code obligations. Ends in a numbered open-decisions register (D-1 to D-27) other docs cite. |
| `pr-strategy.md` | **How m1 actually ships.** Assesses five options for delivering a rewrite while preserving the eight existing PRs as reference, answers definitively what makes a PR permanently readable, and recommends a per-repo PR decomposition with branch names and review focus. |
| `epic-merge-plan.md` | Reference record of the 8-PR tbtc-v2 stack plus standalone keep-core #4238: the verified PR inventory, per-PR extraction guidance, and the `gh-stack` mechanics. No longer a delivery plan (superseded 2026-08-21); `pr-strategy.md` is the live one. |

### Loss-story design
| File | Role |
|---|---|
| `stranding-compensation-proposal.md` | Compensation module design (Tiers 0-1). The **only buildable** loss-story piece under the Decision; Tier 0 doubles as the stranding-frequency evidence instrument that could reopen the exit question. |
| `shortfall-design-space.md` | Who-pays analysis when a wallet dies holding anchors (Spaces A/B/C). Rejects Space A (slashing invariance) and Space B (fungibility); finds Space C (mint < lock) only viable **conditional on an unbuilt `anchorAmount`/`mintedAmount` decoupling** — **not adopted, not scoped** (LTV value and the §4.3 assessment are still open). |
| `exit/` | Emergency-exit design family — **deferred, retained as reference** (see `exit/README.md` for its own index and the Decision block). |

### Cross-cutting
| File | Role |
|---|---|
| `frost-reservations-interaction.md` | Interaction with the separate FROST/Schnorr migration: no forced sequencing, re-anchor as the migration path, one pending settlement-side patch, storage-merge parity risk. |

---

## Reading order

1. `feature-spec.md` — the design (start §1-§4, skim the rest).
2. `exit/README.md` — the Decision + why `Stranded` won (before building anything).
3. `pr-review-followups.md` + `feature-spec.md` §15/§16 — what's open.
4. `shortfall-design-space.md` -> `stranding-compensation-proposal.md` — the loss story.
5. `roadmap.md` — the milestone cut: §1 is m1, §3 is m2, §4 is how m1 ships
   relative to the existing PRs. Then `m1-variant-comparison.md` if the A+/B
   choice is still open.
6. `milestone-inventory.md` — the completeness check before building: what m1
   ships, declares, or builds new, and the open decisions that gate it.
7. `pr-strategy.md` — how the work becomes pull requests, with
   `epic-merge-plan.md` as the extraction reference behind it.
8. `testing-plan.md` -> `timeline-estimate.md` — hardening and schedule.

**Note on duplicated-looking references:** several docs summarize a conclusion
that another doc carries in full (e.g. `feature-spec.md` §17 summarizes
`frost-reservations-interaction.md`; `feature-spec.md` §15 points at
`pr-review-followups.md`). That is intentional progressive disclosure — the
spec stays the single entry point, the deeper doc holds the evidence.

*Draft, 2026-08-21. Kept under `docs/spec/reservations/` — formerly the
`agent-docs/` scratchpad; promoted into version control 2026-08-21.*
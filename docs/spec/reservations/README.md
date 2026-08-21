# UTXO Reservations — Design & Planning Documents

Working documents for the tBTC v2 UTXO-reservation feature (tbtc-v2 PR chain
#1088, #1090-#1096; keep-core #4238). Reverse-engineered from the open/draft
PR stack, not an authoritative protocol spec that the team has published —
parameter values, findings and schedules here are provisional until
governance sign-off and the external audit (see `feature-spec.md` §10, §15).

**Decision anchor (2026-08-21):** the existing **`Stranded`** fallback is the
accepted outcome for a terminated wallet's reservations; the emergency-exit
mechanism is **not built** and retained only as design reference. The
authoritative Decision block (including the re-open triggers) lives in
`exit/README.md`. Every doc in this folder is subordinate to that decision.

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
| `roadmap.md` | **Scope decision.** Milestone-based decomposition for a create-only first release: what ships in m1, what defers to m2, optimal merge order, suggested edits to existing PRs, and the m1 implementation gaps. Source-verified and branch-tagged; §0 records two earlier decisions it reversed. |
| `m1-variant-comparison.md` | **Decision support.** Flat side-by-side of the two m1 designs actually in contention (A+ and B): shared feature set, the single difference (dissolution), measured line counts, pros/cons, and the volume condition that selects between them. Derivation lives in `roadmap.md` §5.2. |
| `timeline-estimate.md` | Schedule: phases, baseline + testing fold-in, testnet round (added 2026-08-21), and the §5 rewrite after the Stranded-decision review. |
| `testing-plan.md` | Test & hardening plan: pre-audit vs during-audit tooling (Foundry invariants, TLA+, multi-signer sim, fork e2e, Certora, etc.), effort, critical-path impact. |
| `epic-merge-plan.md` | Sequencing the 8-PR tbtc-v2 stack plus standalone keep-core #4238 to mergeable/audit-ready state. |

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
4. `shortfall-design-space.md` → `stranding-compensation-proposal.md` — the loss story.
5. `roadmap.md` — the milestone cut and merge order (read before executing
   the merge plan), then `m1-variant-comparison.md` if the A+/B choice is
   still open.
6. `epic-merge-plan.md` → `testing-plan.md` → `timeline-estimate.md` — how it ships.

**Note on duplicated-looking references:** several docs summarize a conclusion
that another doc carries in full (e.g. `feature-spec.md` §17 summarizes
`frost-reservations-interaction.md`; `feature-spec.md` §15 points at
`pr-review-followups.md`). That is intentional progressive disclosure — the
spec stays the single entry point, the deeper doc holds the evidence.

*Draft, 2026-08-21. Kept under `docs/spec/reservations/` — formerly the
`agent-docs/` scratchpad; promoted into version control 2026-08-21.*
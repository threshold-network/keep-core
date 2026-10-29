# Reservations - canonical verification specification

This is the canonical review set for M1 Reservations across contracts, SDK and keep-core.
The text is STE-inspired. Historical documents remain available, but they do not override this set.

## Purpose and limits

Two questions must be reviewed separately:

| Question | Required evidence | A source inspection cannot establish |
|---|---|---|
| Is the design safe and usable? | Economic objectives, safety invariants, liveness assumptions, abuse cases and recovery limits | That a property is desirable merely because the code implements it |
| Does the implementation follow the design? | Exact guards, counter mutations, cross-repo contracts and behavior tests | That a test passed merely because its source exists |

An observed implementation rule is not automatically an approved design requirement.
A legal intermediate state is not automatically a guaranteed eventual outcome.
A timeout deadline does not execute a transaction or release capacity by itself.

M1 includes reserved reveal, acceptance, custody, re-anchor, timeout cleanup and stranding.
D26 adds release to pool before any acceptance request.
D27 adds the consumer-facing SDK path.
M1 does not implement owner redemption, renewal or dissolution of an accepted position.
M2 migration and compensation proposals remain design references, not shipped recovery guarantees.

## Evidence snapshots

| Evidence | Revision | Meaning |
|---|---|---|
| Core contract evidence | tbtc-v2 `9d2086965` | Default for unqualified contract citations |
| Core client evidence | keep-core `f99ef96e3` | Default for unqualified client citations |
| D26 contract and repaired proof tests | tbtc-v2 `d20cf74ba` | Release-to-pool citations identify this revision explicitly |
| D26 client binding and sweep regressions | keep-core `4b95f0a06` | Release binding was added after the original client pin |
| D27 SDK | tbtc-v2 `7aabfc502` | Includes `3d114b15e` and dust/global-count correction `7768a3370` |
| Sepolia sizing/runbook evidence | tbtc-v2 `52f4f3005` | D30: max active 2, max total 4,000,000 sats; historical rehearsal snapshot |
| Acceptance floor | tbtc-v2 `dc5c2ed71` | `amount > minAmount + 2 * txMaxFee`: contract `d4397f208`, SDK `dc5c2ed71` |
| D31 contracts, deploy scripts and SDK | tbtc-v2 `806b1106f` | Hand-off reserve gate, privileged hand-off and the five-argument caps setter; includes the acceptance floor. On the reservations branch, not deployed |
| D31 testnet client | keep-core `d40f6aadc` | D31 bindings, member-proof target fallback and the D28 keeper gauges. On the reservations branch, not deployed |
| D28 keeper and historical wallet discovery | keep-core `9c4e1b221` | Read-only observation, four risk gauges and observation timestamp; includes wallets registered before activation. On the reservations branch, not deployed |
| D28 monitoring | tbtc-v2 `8eb26c2e2` | Existing monitoring entry point, alert classification, Sentry delivery and Testnet4 SDK setup. On the reservations branch, not deployed |
| M1 code as published | tbtc-v2 `4c98e6b82`, keep-core `f03cc61dc` | The stranding policy, the proven-anchor routing gate, the proof-bearing acceptance request and the late-acceptance window as they are on the public reservations branches. Used by the delta review VF-10 to VF-13; on the reservations branches, not deployed |
| M1 spec freeze | tbtc-v2 `44d9c9c6a`, keep-core `f03cc61dc` | The revisions the auditors review. The contract surface is byte-identical to `4c98e6b82` (the only change between them is an unrelated NTT test file), so every `4c98e6b82` citation resolves at the freeze; the keep-core side is unchanged from the published head. Any fix made after the audit re-pins this entry |
| D28 local acceptance and operator setup | run records (no pin) | Per-stage build pins, real local-fork alerts, historical-discovery regression and remaining live actions; not live Sepolia rollout. The run records are named by file without a pin, see below |

Citation syntax: `repo:path:line` uses the default pin above.
`repo@revision:path:line` overrides it.
A branch name is not an immutable evidence reference.
Later source changes require a delta review; they do not silently update these citations.
A pinned citation names the line at that revision only. When a rule changed between the default pin and a later pin, the entry states the rule twice, once per revision, with the revision on each citation, and labels the earlier form as history. A line number taken from one pin is never to be read against another pin's file.
A bare citation such as `Reservation.sol:788` names only a file. `tools/repin.py` resolves it by a unique suffix match.
A relative citation such as `:900` uses the file most recently named in the same table row, list item or paragraph.
A `.md` filename with line numbers but no repository prefix and no pin (`keeper-alert-acceptance.md:81-83`) names a document outside the two evidence repositories: an earlier revision of this set, or a release record. The line numbers are that document's own, and no pin covers them.
Unqualified `keep-core:` and `tbtc-v2:` citations resolve only in a checkout that contains the named commits.
They do not resolve against this repository's `dev`, which holds neither the reservation implementation nor the `TBTC_V2_REF` CI setting that XR-G-1 cites.

Operational state is separate from source state.
A branch, a committed test or a fork artifact does not mean deployment on Sepolia or mainnet.
The live milestone tracker remains `docs/plans/reservations-epic-tracker.md` on the main #4282 branch.
`13-approved-design-deltas.md` records the deployment dependencies and the operator gates; this set does not repeat them.

## Section map

| Section | Stable IDs | Review purpose |
|---|---|---|
| [System model](01-system-model.md) | SM- | Actors, components, storage and allowed transitions |
| [Invariants](02-invariants.md) | INV- | Legal/illegal state relationships, conservation equations, backing and time rules |
| [Parameters and governance](03-parameters-governance.md) | PAR- | Types, units, disabled-cap semantics, sizing and governance order |
| [User stories](04-user-stories.md) | US- | Positive and negative paths, actors, guards, events and reverts |
| [Edge cases](05-edge-cases.md) | EC- | Equality boundaries, races, late proofs, reorgs and state changes |
| [Failure modes](06-failure-modes.md) | FM- | Triggers, detection, money at risk and available mitigation |
| [Disaster recovery](07-disaster-recovery.md) | DR- | Required signers, explicit transactions, deadlines and unrecoverable states |
| [Threat model](08-threat-model.md) | TM- | Assets, adversaries, capabilities, assumptions and residual attacks |
| [Cross-repo contract](09-cross-repo-contract.md) | XR- | ABI, event address/indexing, proposal validation, client timing and metrics |
| [Accepted risks](10-accepted-risks.md) | AR- | Recorded decisions, bounded exposure and revisit triggers |
| [Traceability](11-traceability.md) | All prefixes except VF- | One row per review item, evidence location and test/rehearsal reference. `12-finding-adjudication.md` owns the VF- IDs and they have no matrix row |
| [Finding adjudication](12-finding-adjudication.md) | VF- | Confirmed observations, rejected claims and explicit evidence limits |
| [Approved design deltas](13-approved-design-deltas.md) | D31, D28 | Integration evidence at later pins, local acceptance and remaining live deployment limits |

The larger chapters use linked detail files. Their IDs remain stable across file moves.
The traceability table identifies the actual file that owns each item.

## Review method

1. Read the model, scope and trust assumptions.
2. Check whether each safety requirement prevents a concrete bad outcome.
3. For a liveness claim, identify every required signer, caller, chain and free-capacity assumption.
4. Walk each transition through its state guards and all counter changes.
5. Check the same rule at request, signing, Bitcoin confirmation, proof and cleanup time.
6. Exercise the positive path and the rejection path at each boundary.
7. Check SDK, client and contract rules independently. A preflight does not reserve capacity.
8. Review every failure with a recovery owner, admissible transaction and remaining deadline.
9. Record deviations and rejected findings. Do not silently convert them into accepted design.
10. Run the named behavior checks against the identified snapshots before approving implementation readiness.

## Evidence status

| Label | Meaning in this set |
|---|---|
| `holds` | The inspected code implements the stated rule; the section links supporting behavior tests where identified |
| `holds-untested` | The code supports the rule, but the exact boundary or end-to-end claim lacks a named behavior check |
| `violated` | An explicit stated agreement does not match the inspected source; this is not a claim that CI was run |
| `design-only` | A future requirement or procedure is not implemented at the named snapshot |
| `unknown` | Deployment facts, intent or evidence are insufficient to decide |
| `model` | Descriptive actor/component/storage row, not a tested correctness assertion |
| `pointer` | An alias to existing review items; it adds no new coverage |

The chapter-local status text gives the qualification for each item.
`holds` is not formal verification, deployment approval or a runtime pass.
All code-backed observations must retain their exact source pin.
Rehearsal evidence belongs to a specific artifact and scenario, not to an entire subsystem.

## Maintenance and publication

- Update the owning item when a guard, signature, event, counter or user promise changes.
- Rebuild `11-traceability.md` from the owning items in the same documentation change: `python3 tools/traceability.py . > 11-traceability.md`. Do not edit its rows by hand.
- Update the inventory classification in the same documentation change.
- Before each commit, run `python3 tools/traceability.py . --check` and `python3 tools/smoke.py . <tbtc-v2 repo> <keep-core repo>` from this folder. Both must exit 0. The smoke check needs `markdown-it-py` (`pip install --require-hashes --only-binary :all: -r tools/requirements.txt`). After you change `smoke.py` or `traceability.py`, run `python3 -m unittest test_smoke test_traceability` from `tools/`.
- `traceability.py --check` exits 1 on a structural problem: an ID with several owners, an owner without a status, a summary row whose status differs from its owner, or an ID that no chapter owns (`unknown ID`). It also exits 1 when `11-traceability.md` differs from the derivation.
- `traceability.py --check` also lists, under `check_references`, the Go checks (`Test`, `Fuzz` and `Benchmark` functions and `_test.go` files) that an evidence field names but that are not on the checked-out branch. Those checks are `missing` on this branch; they live at the pinned commits that the owning entry cites. A `missing` check does not change the exit code. The smoke check fails when a pinned citation is absent at its pin.
- `traceability.py` also checks the keep-core registry set in `docs/spec/keep-core/verification` (the `KC-` ID space). For a set whose directory has no `11-traceability.md` the matrix comparison is skipped, a critical entry with no checker fails `--check`, and `Checkers` and `Fails on` fields are resolved like evidence fields. See `docs/spec/keep-core/verification/README.md`.
- The smoke check reads explicit citations only. To check bare and relative citations too, run `tools/repin.py` with the same commit as `<old>` and `<new>`: once with `--default` for each repository, and once for each explicit pin in the set. Each run must report empty `unresolved` and `relative_without_file` lists.
- Quote check (warnings only). When a double-quoted string directly touches an explicit citation in the same paragraph, table cell or list item (quote before citation, or citation before quote), the smoke check searches the cited lines for it. It converts Unicode dashes and quotes in the source to ASCII, collapses whitespace, and lets `...` in a quote match any text. A quote that is not found is listed in `warnings` and counted in `quote_warnings`. It never changes the exit code and never hides a source, link, table or traceability error. A paraphrase, or a quote whose citation points at a neighboring line, stays a warning until you fix the quote or the line number.
- The quote check does not verify every quote. A quote separated from a citation by words, a sentence end or `;` is not associated; it is only counted in `quotes_unassociated`. Where a quote touches two citations, it passes if either contains it. Each alternate quote (`"A" or "B"`) is checked separately against the adjacent citations. `quotes_checked` counts only the quoted strings that were associated and compared.
- Partial mode: `python3 tools/smoke.py . - <keep-core repo>` skips every `tbtc-v2` citation and does not read that repository. The report lists `skipped_repositories` and `citations_skipped`. `explicit_pinned_citations_checked` counts only the citations that were checked, and quotes next to skipped citations are counted in `quotes_skipped`. The `reservations-spec` workflow runs the full check instead: it checks out `threshold-network/tbtc-v2` into a second directory, so both repositories are public and no cross-repository secret is needed. Partial mode remains for a local run that has only one checkout.
- The workflow checks out the full git history so that pinned keep-core commits resolve. A pin that is not in the fetched history is reported as an absent source.
- Write a citation that quotes another document's line numbers with that document's own pin (for example `tbtc-v2@9f8f5ef1:...`), so that a later re-pin cannot move it.
- Preserve historical findings and decisions, but label their scope and superseding evidence.
- The auditors review the freeze SHA on the public heads. The evidence entry pins that SHA, and any fix made after the audit re-pins it.
- To move citations to a new commit, run `python3 tools/repin.py . <tbtc-v2|keep-core> <repo path> <old> <new> [--default]` without `--write` first. Read its `pinned_to_old`, `made_explicit`, `unresolved` and `relative_without_file` lists. `--write` refuses to run while `unresolved` or `relative_without_file` is not empty.
- When a pinned commit is replaced, move every citation with `tools/repin.py`, set the new default pins in `tools/smoke.py` (`PINS`) and in the evidence table above, rebuild the matrix and run both checks.
- Do not merge an old plan's scope into M1 by implication.

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
| M1 code as published (history) | tbtc-v2 `4c98e6b82`, keep-core `4ced1c839` | The stranding policy, the proven-anchor routing gate, the proof-bearing acceptance request and the late-acceptance window as they were on the public reservations branches at this head. The freeze moved first to `c8350d109` and then to the merged tree below, and the live citations in this set name that pin; the citations that stay at `4c98e6b82` are this row, the VF-10 to VF-13 delta record and the sentences that state where the code was read when the review ran. |
| M1 spec freeze | tbtc-v2 `c8350d109`, keep-core `4ced1c839` | The revisions the auditors review. This row named `44d9c9c6a` until 2026-10-30, which was byte-identical to the published head `4c98e6b82`; the pin is now the tree the three remaining tbtc-v2 pull requests merge to. `4c98e6b82..c8350d109` added the `MAX_RESERVATION_DISSOLUTION_DELAY` bound (730 days) as a new internal constant in `Reservation.sol`, its two enforcement points and the matching doc comments; the merge adds tbtc-v2#1178's correction of the `dissolutionEligibleAt` reader comment in `Reservation.sol` (comments only), tbtc-v2#1179's `BridgeGovernance.requestReservationReanchor(uint256,bytes20)` and `pendingReservationCapsUpdate()` (two new external functions, no change to any existing signature, selector, event or storage slot) and tbtc-v2#1180's Foundry suites, Medusa handlers and surface specification (test files and one JSON, no contract code), plus tbtc-v2#1179's own deflake of `Bridge.Governance.test.ts` at its final head (`f176768f`: one `await helpers.time.increaseTime(1)`, ten comment lines and a blank, inside one `it`; no contract bytecode, and no line the spec cites; its highest citation into that file is `:4889`, below the insertion at `:4972`). Every tbtc-v2 citation in this set was re-resolved line by line at that tree, the one citation whose range ran past the end of its file was corrected, and the claims the changes falsified were rewritten; the keep-core citations were re-pinned from `f03cc61dc` to `4ced1c839` on 2026-10-29, and that range carries keep-core #4365, the client-side fixes and the test, fuzz and audit-record commits around it, every keep-core citation re-resolved at that pin. Any fix made after the audit re-pins this entry |
| D28 local acceptance and operator setup | run records (no pin) | Per-stage build pins, real local-fork alerts, historical-discovery regression and remaining live actions; not live Sepolia rollout. The run records are named by file without a pin, see below |

Citation syntax: `repo:path:line` uses the default pin above.
`repo@revision:path:line` overrides it.
A branch name is not an immutable evidence reference.
Later source changes require a delta review; they do not silently update these citations.
A pinned citation names the line at that revision only. When a rule changed between the default pin and a later pin, the entry states the rule twice, once per revision, with the revision on each citation, and labels the earlier form as history. A line number taken from one pin is never to be read against another pin's file.
A bare citation such as `Reservation.sol:788` names only a file. `tools/repin.py` resolves it by a unique suffix match.
A citation written from 2026-10-30 on names its revision whenever the cited file changed between the previous freeze pin and the current one: `tools/pin_rule.py` fails a citation added after the baseline commit in `tools/pin-rule.json` that relies on the default pin for such a file. Citations already in the set are not re-checked; see the maintenance notes below.
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
- Before each commit, run `python3 tools/traceability.py . --check`, `python3 tools/smoke.py . <tbtc-v2 repo> <keep-core repo>` and `python3 tools/pin_rule.py <keep-core repo> <tbtc-v2 repo>` from this folder. All must exit 0. The smoke check needs `markdown-it-py` (`pip install --require-hashes --only-binary :all: -r tools/requirements.txt`). After you change `smoke.py`, `traceability.py` or `pin_rule.py`, run `python3 -m unittest test_smoke test_traceability test_pin_rule` from `tools/`.
- Pin rule for citations written in new or changed lines. `tools/pin_rule.py` reads the baseline commit and the two pin ranges from `tools/pin-rule.json`, takes the `.md` lines added since the baseline, and fails when one of them cites a file that changed between the previous freeze pin and the current one without naming a revision. Both a bare citation and a `repo:` citation with no `@revision` rely on the default pin, so the line means different code once the file changes. A pinned citation, a citation into a file that did not change in the range, and text that predates the baseline all pass. Relative citations are checked through the file named by their owner. The citations already in the set are deliberately not checked: the 2026-10-30 review measured 18 wrong line numbers among 1,701 bare citations into changed files, and a cross-pin check flagged 1,505 of those 1,701, so the hit rate is about one and a half percent and the readable half of the job stays a documentation pass. The rule stops the defect from growing.
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
- When a pinned commit is replaced, move every citation with `tools/repin.py`, set the new default pins in `tools/smoke.py` (`PINS`) and in the evidence table above, set the new `previous` and `current` pins in `tools/pin-rule.json`, rebuild the matrix and run every check. Move the `baseline` in that file forward once the move is published, so citations written against the old pins are no longer treated as new text.
- Do not merge an old plan's scope into M1 by implication.

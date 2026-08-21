# PR attribution map (measured 2026-08-21)

All figures from `git` against a `tbtc-v2` clone
with every branch ref fetched. Diffs are `origin/<base>...origin/<head>`, so each
row is that PR's own contribution relative to its immediate predecessor.

## 1. Per-PR diffstat, split by kind

| PR | Head branch | prod Solidity | Solidity tests | deploy | docs | other | files |
|---|---|---|---|---|---|---|---|
| #1088 | `feat/utxo-reservation-core` | +3393 -34 | +3925 -4 | +71 -0 | - | +42 -0 | 29 |
| #1090 | `feat/utxo-reservation-router` | +874 -315 | +2408 -127 | +199 -84 | +426 -0 | +35 -13 | 29 |
| #1091 | `feat/utxo-reservation-settlement` | +3594 -1266 | +5724 -507 | +117 -12 | +438 -15 | +90 -9 | 43 |
| #1092 | `feat/utxo-reservation-renewal` | +351 -115 | +631 -122 | +23 -6 | +51 -21 | - | 14 |
| #1093 | `feat/utxo-reservation-backing` | +457 -23 | +961 -23 | +10 -3 | - | - | 16 |
| #1094 | `feat/utxo-reservation-guards` | +537 -39 | +2243 -9 | - | +4 -2 | - | 13 |
| #1095 | `docs/utxo-reservation-release` | - | +4 -1 | +12 -8 | +421 -22 | - | 5 |
| #1096 | `feat/utxo-reservation-partial-redemption` | +696 -92 | +2441 -75 | - | +108 -40 | +14 -0 | 17 |

Two things the totals hide:

- **`#1091` is the largest and most destructive PR in the stack**: +3594 -1266
  production lines. It deletes more production Solidity than `#1092`, `#1093`
  and `#1094` add put together. Anything extracted from `#1088` or `#1090` must
  be taken in its post-`#1091` form, not as originally written.
- **`#1095` adds no production Solidity at all.** It is docs plus one test line
  plus deploy-script edits. As an extraction source it contributes only the
  frozen-spec and runbook prose.

## 2. Branch drift: three branches are behind their bases, not one

`git rev-list --left-right --count origin/<base>...origin/<head>`:

| PR | commits behind base | commits ahead | base commits missing from head |
|---|---|---|---|
| #1088 | **2** | 25 | `502cd398 fix(security): replace CryptoJS key encryption (#1098)`, `ca49fec3 chore: mark stale treasury fee values as historical/test-only` |
| #1090 | 0 | 10 | - |
| #1091 | **13** | 37 | `48bc9f03 fix(deploy): declare Bridge as dependency of ReservationVault`, `c5013bf4 fix(bridge): thread isRetry through router surface`, `9aeefc6d fix(bridge): reconcile router surfaces with the rebased reservation core`, `16bf7e41 fix(bridge): harden reservation router wiring` |
| #1092 | **5** | 12 | `f32c8cf3 Apply prettier formatting and fix await-in-loop lint`, `10899916 Annotate RFC 13 with this stack's deferred scope`, `1ee595da Rename veto event parameter to cover both key spaces`, `6d086af7 Require the router before activating the reservation vault` |
| #1093 | 0 | 12 | - |
| #1094 | 0 | 15 | - |
| #1095 | 0 | 16 | - |
| #1096 | 0 | 17 | - |

**This corrects `epic-merge-plan.md` §0.1**, which records `#1090` as the only
CONFLICTING PR and the only rebase needed. That was true when written; it is not
true now. `#1090` has since been rebased (0 behind, and it contains the `#1102`
fold - see §3), and the staleness has **moved up the stack** to `#1091` and
`#1092`.

Note the distinction: GitHub's CONFLICTING label means textual merge conflict,
whereas "behind base" means the head branch lacks commits its base has. A branch
can be behind without conflicting, and it is still a correctness problem for
extraction, because reading the stale branch shows pre-fix code.

`#1088` being 2 behind `reservations-epic` is a different case: the epic branch
moved forward with an unrelated security fix (`#1098`). That is normal drift, not
a stack problem, but it does mean `reservations-epic` is not a frozen base.

## 3. The `#1102` fold reaches only the bottom two branches

`#1102` is commit `3566e059 fix(reservation): address multi-agent review findings
on #1088 (#1102)`, a squash merge (single parent `3d4335d7`).

Ancestry checks for `3566e059`:

| Branch | Contains the fold? |
|---|---|
| `feat/utxo-reservation-core` (#1088) | **yes** |
| `feat/utxo-reservation-router` (#1090) | **yes** |
| `feat/utxo-reservation-guards` (#1094) | **no** |
| `feat/utxo-reservation-partial-redemption` (#1096) | **no** |

**This is the most consequential finding in this document.** The
`m1-b-implementation.md` provenance block states its citations are
"source-verified on `feat/utxo-reservation-guards`". That branch does **not**
contain `#1102`'s 30 review fixes. So every line number cited from the guards
tip is a line number in code that predates the review fixes, and any behaviour
`#1102` changed is cited in its pre-fix form.

This follows from §2: the fold reached `#1090` (which was rebased over it), but
`#1091` is 13 commits behind `#1090`, so the fold stops there and nothing above
`#1091` has it.

What `#1102` changed, production files only (`git diff --numstat 3566e059^1
3566e059 -- solidity/contracts/`):

| File | Change |
|---|---|
| `bridge/Reservation.sol` | +342 -95 |
| `bridge/RedemptionWatchtower.sol` | +88 -16 |
| `bridge/BridgeState.sol` | +62 -21 |
| `bridge/Redemption.sol` | +61 -19 |
| `vault/ReservationVault.sol` | +55 -15 |
| `bridge/Bridge.sol` | +36 -5 |
| `bridge/BridgeGovernanceParameters.sol` | +22 -2 |
| `bridge/BridgeGovernance.sol` | +11 -3 |
| `bridge/WalletProposalValidator.sol` | +7 -3 |
| `test/BridgeStub.sol` | +1 -11 |

+685 -190 across 10 production files. This is not a cosmetic fold.

## 4. Correction: the `reservationsByAnchorUtxo` story in the docs is wrong

`m1-b-implementation.md` §4.3 states that `#1091` writes the mapping, `#1094`
writes it again, and "`#1102` removed it from the merged base in favour of
`spentMainUTXOs`", concluding that "two write sites and one removal must be
reconciled".

Measured, by grepping each branch:

| Branch | `reservationsByAnchorUtxo` in `BridgeState.sol` |
|---|---|
| `feat/utxo-reservation-core` (#1088) | 0 hits |
| `feat/utxo-reservation-router` (#1090) | 0 hits |
| `feat/utxo-reservation-settlement` (#1091) | 1 hit |

`reservationsByAnchorUtxo` **does not exist on `#1088`'s branch at all.** It is
introduced by `#1091`. Therefore `#1102`, which merged into `#1088`'s branch,
cannot have removed it - there was nothing there to remove. There is no removal
to reconcile.

Separately, `spentMainUTXOs` is a **pre-existing Bridge registry**, not a
replacement introduced by `#1102`. On `#1088`'s branch it already has 6 mentions
in `Reservation.sol`, which writes to it at `:1454` and `:1510` and documents it
at `:66` as "the existing registry of honestly-spent" outpoints.

So the two are not competing designs. They are different things: `spentMainUTXOs`
is the Bridge's honestly-spent-outpoint registry that reservations write into, and
`reservationsByAnchorUtxo` is a reverse index from anchor outpoint to reservation
key that `#1091` adds. The real reconciliation item is narrower: two write sites
for the reverse index (`#1091` and `#1094`), both still present at the guards and
partial tips.

## 5. Extraction hazards

Ranked by how expensive it is to get wrong.

1. **Extracting from the guards tip silently drops `#1102`.** Anything taken
   from `Reservation.sol`, `ReservationVault.sol`, `BridgeState.sol`,
   `Redemption.sol`, `RedemptionWatchtower.sol`, `Bridge.sol`,
   `BridgeGovernance*.sol` or `WalletProposalValidator.sol` at the guards tip is
   pre-`#1102` code. Either extract those files from `#1090`'s tip and re-apply
   the upper-stack changes, or rebase `#1091` first. This must be settled before
   any extraction begins.
2. **`#1091` rewrites `#1088` heavily** (-1266 production lines). Never extract
   the `#1088` form of anything `#1091` touched; the single-phase mechanics were
   replaced by the two-phase machine.
3. **`#1093` reworks the backing model** that `#1091` established, per the H-04
   finding. Extract the `#1093` form of anything touching `mintedAmount` or
   `anchorAmount`.
4. **The reverse anchor index has two write sites** (`#1091`
   `ReservationProofs.sol:465` region and `#1094`'s stranding write). Both must
   be carried, because stranding is variant B's only position-closing *entry point*.
5. **`#1092`'s expiry model is structural, not additive.** Its `window < term`
   arithmetic and dissolution-eligibility snapshotting are depended on by
   `#1093`+, so `#1092` cannot be skipped even though renewal is deferred; the
   snapshot semantics are what make governance changes non-retroactive.
6. **`#1095` has no production code**, so treat it purely as a docs source.

## 6. Per-PR extraction verdict for m1

| PR | Contribution | m1 verdict |
|---|---|---|
| #1088 | Core data model, `ReservationVault`, original single-phase mechanics | **Extract, post-`#1102`, post-`#1091` form** - the storage foundation, but its mechanics were replaced |
| #1090 | Router delegatecall + EIP-170 workaround, RFC 13 | **Extract** - m1 keeps a minimal router, and this is the only source for the delegatecall pattern and its four invariants |
| #1091 | Two-phase authorize-then-prove settlement | **Extract** - this is m1's core; highest-value source in the stack |
| #1092 | Bounded renewal, strict expiry semantics | **Extract the expiry/snapshot half, skip the renewal half** - renewal is m2, but the snapshot semantics are structural |
| #1093 | Claim-equals-anchor backing, financed in-kind fees | **Extract** - the backing invariant is m1, and re-anchor's in-kind fee is live in m1 |
| #1094 | Designated-wallet binding, pending-deposit guard, stranding, monitoring | **Extract** - stranding is m1's only close path |
| #1095 | Docs and one test line | **Reference only** - no production code |
| #1096 | Partial reserved redemption (1-in-2-out) | **Skip** - redemption is m2 in whole and in part |
| #1102 | 30 review fixes on `#1088` | **Extract, mandatory** - and note it is absent above `#1091` |
| keep-core #4238 | Wallet-side two-phase type layer, no executor (C-8) | **Build on it** (D-25) - the type layer is reusable; m1 supplies the executor it lacks. Corrected 2026-08-21: this cell previously said "Rewrite - written against the superseded single-phase design", which C-8 refutes and which contradicted this row's own description cell |

## Open questions

1. **Rebase `#1091` and `#1092`, or extract around them?** Rebasing makes the
   guards tip trustworthy as a single extraction source, which is worth a lot
   given how many docs already cite it. Extracting around the gap means every
   extraction must consult two branches per file. Recommendation is to rebase
   first, but it touches reviewed logic in three PRs, so it is a human call.
2. **Is `reservations-epic` the right base for m1, given it now carries an
   unrelated security fix (`#1098`)?** A fresh branch from current `main` would
   be cleaner for a rewrite, but loses the epic branch's isolation property.
3. **Should the docs' guards-tip citations be re-verified against a rebased
   tip?** Every line number in `m1-b-implementation.md` and `feature-spec.md`
   was read from a pre-`#1102` tree. The line numbers are probably still close,
   but "probably" is not a standard this doc set has been holding itself to.

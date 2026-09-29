# UTXO Reservations — Testing & Hardening Plan (pre-audit)

Status: DRAFT. Companion to `feature-spec.md` and `pr-strategy.md`
(the live delivery plan; `epic-merge-plan.md` is the superseded stack record). Grounded in a direct inspection of both
repos' tooling, originally surveyed 2026-08-19 and re-verified against M1 code on 2026-09-28.

**Current routing — read this first.** Tier 1 (§3) lands directly in the m1
rewrite itself, not retrofitted into the eight stack PRs; Tier 2 stays the
concrete content of the release runbook's fork-dry-run checklist item; Tier 3
runs alongside the external audit. Full routing detail and what variant B
changes about test content: §4.

**Superseded in part, 2026-08-21.** m1 is now variant B, an essentials-only
rewrite with **no dissolution, no redemption, no renewal** (`roadmap.md` §1).
Any lifecycle sequence below ending in `dissolve`/`renew`/`partial-redeem`,
and the assumption that tests land inside the eight stack PRs, are stale
wherever they appear — §4 carries the corrected routing and the invariants
B's launch gates add.

**Re-verified 2026-09-28 against M1 code** (tbtc-v2 `reservations-upgrade` @
`9f8f5ef1`, keep-core `reservations-epic` @ `f66f11240`): §1-§2's tooling
survey below does **not** all still stand as originally written — file
names, function names, line/function counts, and §2's central "real gap"
conclusion have all moved since the 2026-08-19 survey. Both sections are
corrected in place below; §3's Tier 1 items are individually annotated with
their current status where the gap they targeted has since closed.

## 0. Direct answers

- **Fuzzers for tbtc-v2 contracts: yes, add them.** Zero fuzzing exists
  today (confirmed below) on a contract that custodies real BTC value with
  named global invariants (claim≡anchor, supply conservation, storage
  append-only). This is exactly the class of property that fuzzing catches
  and example-based Mocha tests don't.
- **Formal verification for contracts: partial, narrow scope, not the
  whole state machine.** Full CVL specs for all 8 PRs' worth of logic is
  too slow to fit before audit and duplicates what an auditor will do
  anyway. Worth it for the ~4 highest-value global invariants only,
  run in parallel with (not gating) the external audit.
- **Fuzzers for keep-core: yes, but narrow and cheap.** Extend the
  project's *existing* native Go fuzz pattern (`pbutils.FuzzUnmarshaler`,
  already used for every other wallet-action proposal type) to reservation
  proposals once wired. This is a two-line addition to an established
  convention, not new tooling.
- **Formal verification for keep-core: not code-level — protocol-level.**
  There's no mature formal-verification framework for Go client code
  analogous to Certora. The real leverage is a **TLA+ model of the
  two-phase protocol itself** (m1 B scope: request -> authorize ->
  prove/settle -> re-anchor -> strand, with SPV-maintainer-stall and timeout
  transitions; renew/dissolve/veto are M2-only, see §4), checked *before*
  finishing the keep-core wiring — it catches cross-system races between the
  contract, the watchtower, and the wallet that no amount of single-repo
  unit testing can see, because each repo's tests can only see its own
  side.

## 1. Current tooling — tbtc-v2 (Solidity), surveyed 2026-08-19, file inventory re-verified 2026-09-28 against M1 code

Checked out at `feat/utxo-reservation-partial-redemption` (stack tip).

- **Stack**: Hardhat + Mocha + Chai only. `solidity/package.json`
  devDependencies: `@nomiclabs/hardhat-waffle`, `chai`, `chai-as-promised`,
  `@types/mocha`. No Foundry, no Echidna, no Certora, no property-based
  testing library, no mutation testing tool. (Re-verified 2026-09-28: still
  true — no `foundry.toml` anywhere in the repo.)
- **CI** (`.github/workflows/contracts.yml`): job
  `contracts-build-and-test` runs `yarn test` then `yarn test:integration`;
  job `contracts-slither` runs `slither --hardhat-artifacts-directory
  build .`. No fuzz/invariant/formal-verification job exists.
- **Slither** (`solidity/slither.config.json`) is the only static-analysis
  tool in the pipeline — it's a Trail of Bits tool, which matters because
  Echidna and Gambit (recommended below) are from the same toolchain and
  slot in with zero new vendor relationship.
- **Fork testing**: `hardhat.config.ts` has forking wired via
  `FORKING_URL`, but no reservation test uses it — confirmed no fork test
  exists for this feature.
- **Reservation-specific tests, re-verified 2026-09-28 against M1 code**
  (14 files, all pure Hardhat/Mocha/Chai, 0 Foundry-style
  `invariant_`/`testFuzz_` functions found anywhere — the file list below
  replaces the stale 2026-08-19 list, which named files that no longer exist
  under those names: `ReservationRouter.test.ts`, `ReservationBacking.test.ts`,
  `ReservationGuards.test.ts`, `ReservationPartial.test.ts`,
  `ReservationInvariants.test.ts`):
  - `solidity/test/bridge/Bridge.RouterStorageParity.test.ts` (239 lines) and
    `Bridge.RouterSelectorDisjointness.test.ts` (69 lines) — the delegatecall
    storage-layout-parity and selector-collision checks that replaced the old
    `ReservationRouter.test.ts`.
  - `Reservation.test.ts` (1919 lines) and `ReservationProofs.test.ts`
    (1886 lines) — core lifecycle and proof-submission mechanics.
  - `Bridge.ReservationAbiSnapshot.test.ts` (35),
    `Bridge.ReservationAcceptanceAuthorization.test.ts` (686),
    `Bridge.ReservationCaps.test.ts` (337),
    `Bridge.ReservationOccupancy.test.ts` (268),
    `Bridge.ReservationSettlement.test.ts` (614),
    `Bridge.ReservationSourceAnchorBinding.test.ts` (547),
    `Bridge.ReservationStranding.test.ts` (841), and
    `Bridge.ReservationStrandingLibrary.test.ts` (1694) — acceptance
    authorization, caps, occupancy, settlement/re-anchor grinding,
    source-anchor binding, and stranding, each split into its own file.
    Some of these (e.g. `Bridge.ReservationSettlement.test.ts`'s "cumulative
    re-anchor fee exposure (accepted regression)" characterization test —
    see `pr-review-followups.md`'s Resolution section) are named-invariant,
    example-based checks in the same style the 2026-08-19 survey flagged
    under the old `ReservationInvariants.test.ts` name — still not
    property-based/fuzzed, still the best conversion candidate for Tier 1
    item 1 below.
  - `solidity/test/deploy/98_generate_reservation_mainnet_calldata.test.ts`
    — deploy-script test (the 2026-08-19 survey named
    `95_deploy_reservation_vault.test.ts`, which does not exist under that
    name).
  - `solidity/test/vault/ReservationVault.test.ts` (834 lines) — vault-side
    unit tests (initiation fee, in-kind fee financing/debt, `sweepFees`
    debt-then-target-floor behavior, `repayInKindFeeDebt`), omitted from
    the 2026-08-19 survey and from this list until this pass; not counted
    among the "12" in earlier drafts of this bullet.

## 2. Current tooling — keep-core (Go), surveyed 2026-08-19, re-verified 2026-09-28 against M1 code

Checked out at `feat/utxo-reservation-wallet-support` originally;
re-verified against `reservations-epic` @ `f66f11240` (28 commits ahead,
including #4274/#4276-#4280/#4324).

- **Stack**: `testify` (indirect dep only, `go.mod:106`), `google/gofuzz`
  (**direct** dep, `go.mod:51`, used by the `pbutils` fuzz harness — the
  2026-08-19 survey called this indirect too; it was already wrong then).
  No ginkgo/gomega, gomock, gopter, or `pgregory.net/rapid`.
- **Native Go fuzzing**: real `func Fuzz*` functions exist only in
  `pkg/internal/pbutils/pbutils.go` (`FuzzUnmarshaler`, `FuzzFuncs`,
  `fuzzBigInt`, `fuzzEphemeralPublicKey/PrivateKey`, `fuzzG1`/`fuzzG2`) —
  this is a real, working fuzz harness the project already trusts, and
  every existing wallet-action proposal type calls it. **Reservation
  proposals now have this coverage too** (closing Tier 1 item 4 below):
  `pkg/tbtc/marshaling_test.go` has
  `TestFuzzCoordinationMessage_MarshalingRoundtrip_WithReservationAnchorProposal`
  and `...WithReservationReanchorProposal`, added in #4277 alongside the
  switch from JSON-placeholder to real protobuf reservation proposal types.
- **CI** (`.github/workflows/client.yml`): `client-build-test-publish` runs
  `gotestsum -- -timeout 15m -coverprofile=... ./...` (no `-race` flag);
  `client-integration-test` runs `gotestsum -- -timeout 20m
  -tags=integration ./...` but is gated to non-PR events only. Exactly 2
  files repo-wide use the `//go:build integration` tag
  (`pkg/bitcoin/electrum/electrum_integration_test.go`,
  reservations. (Re-verified 2026-09-28: unchanged, still true.)
- **Reservation-specific tests — substantially more than the 2026-08-19
  survey found, and now spanning three packages, not one:**
  - `pkg/tbtc/reservation_test.go` (1073 lines, 12 `Test*` functions — the
    2026-08-19 survey's 9-function, 770-line count, and its
    `TestAssembleReservedRedemptionTransaction`/
    `TestAssembleReservationDissolutionTransaction` names, are both stale;
    redemption and dissolution transaction assembly are out of m1 B scope
    and neither function exists). Current functions: enum/state parse
    tests, `TestReservationProposals_MarshalingRoundtrip`,
    `TestReservationProposals_UnmarshalRejectsInvalidPayloads`,
    `TestReservationProposals_MarshalRejectsZeroValues`,
    `TestAssembleReservationTransactions_InputValidation`,
    `TestAssembleReservationTransactions_FeeBoundaries`,
    `TestAssembleReservationTransactions_HappyPathShape`,
    `TestReservationAnchorAction_Execute`,
    `TestReservationReanchorAction_Execute`.
  - `pkg/tbtcpg/reservation_acceptance_test.go` and
    `reservation_reanchor_test.go` (task-level tests against the real
    `newLocalBitcoinChain()` harness, plus dedicated metrics test files) —
    new since the 2026-08-19 survey, landed with the executor in #4274.
  - `pkg/maintainer/spv/reservation_acceptance_proof_test.go`,
    `reservation_reanchor_proof_test.go`,
    `reservation_action_timeout_watch_test.go`,
    `reservation_stale_deposit_watch_test.go`,
    `reservation_stranding_watch_test.go`, `reservation_proof_loop_test.go`,
    and `reservation_wiring_test.go` — also new since 2026-08-19, landed
    with the watcher wiring in #4276.
  - `pkg/chain/ethereum/tbtc_test.go` no longer asserts `"not supported
    yet"` anywhere — the 5 previously-stubbed Ethereum methods the
    2026-08-19 survey found are now real bindings (20+ Reservation methods
    in `pkg/chain/ethereum/tbtc.go`, with real ABI encode/decode helpers),
    landed in #4274. `pkg/clientinfo/performance_test.go` still adds
    reservation action types to an expected-metrics list, unchanged.
- **The real gap — closed, not open, as of #4274/#4278/#4279.** The
  2026-08-19 survey found zero references to `Reservation` in
  `coordination_test.go`/`node_test.go` and concluded the coordination/
  dispatch integration wasn't wired. At `f66f11240`, `coordination_test.go`
  has 50 case-insensitive `Reservation` hits including
  `TestCoordinationExecutor_Coordinate_ReservationProposals` (a 3-operator,
  real-broadcast-channel, real-leader-election simulated integration test —
  this **is** the "multi-signer simulated integration test" Tier 2 item 7
  below asks for, landed in #4279) and
  `TestCoordinationExecutor_GetActionsChecklist_Reservations`; `node_test.go`
  has 7 hits including
  `TestProcessCoordinationResult_ReservationAnchorRoutesToHandler` and
  `...ReservationReanchorRoutesToHandler`. Production code
  (`coordination.go:713-715`, #4278) unconditionally appends
  `ActionReservationAnchor`/`ActionReservationReanchor` to the checklist;
  handler dispatch is wired in `node_coordination.go`/`node_proposals.go`.
  This closes Tier 1 item 3 below.

## 2.5. Direct validator coverage (I-7) — landed

The deferred direct-validator-test row (`m1-keep-core-readiness/01-gap-analysis.md`
Minor row "no direct unit tests"; `03-delta-changes.md` S8 row, Story S8) is closed
by a **go-ethereum in-memory EVM validator
harness** (`pkg/chain/ethereum/tbtc_validator_harness_test.go`; Phase 2 work,
dedicated agent, I-7): it deploys
the real, unmodified `WalletProposalValidator` into an in-memory go-ethereum
EVM (`core/vm/runtime` over a single shared `StateDB`) backed by a stub
Bridge, so the two real chain-calling wrappers in
`pkg/chain/ethereum/tbtc.go` (contract call, error wrap, `valid == false`
branch, and revert-reason propagation) get direct tests for the first time.
The `core/vm/runtime` backend was chosen because `ethclient/simulated`
could not be linked by the test binary on Go 1.24: it pulls in
`internal/debug`'s `fjl/memsize` dependency, whose reference to
`runtime.stopTheWorld` trips the Go 1.23+ linkname restriction that
CI's plain `go test` (no `-checklinkname=0`) enforces.

**P0 lesson recorded 2026-09-28:** permissive validator fakes in the
unit and multi-signer tests hid a validate-before-request ordering bug —
keep-core called
`ValidateReservationAnchorProposal`/`ValidateReservationReanchorProposal`
against a predicted nonce *before* the required authorization action
existed (and in the acceptance case even tried to call
`RequestReservationAcceptance` operator-side, which on-chain is
depositor-only), so neither wallet-side flow could ever reach signing at
the pinned tips. The tests passed because the fakes returned "valid"
without enforcing the on-chain preconditions, and the multi-signer test
injected ready-made proposals. The fix (in `fix/m1-cross-repo-review`)
replaces the permissive fakes with **strict validator fakes** that
enforce the real preconditions (a Pending action must exist at the
proposal's nonce; acceptance consumes the depositor's Pending Acceptance
action instead of requesting one; re-anchor requests - recording the
transaction hash for the cross-round in-flight receipt check - wait up to 6
blocks in the same round for the request to be mined, re-read the reservation
and the action at its real nonce, then validates). Rule going forward: any fake
standing in for an
on-chain validator must enforce the same state preconditions the contract
does, or the test proves nothing about ordering.

## 3. Recommendations, ranked by bang-for-buck

### Tier 1 — cheap, high value, do before anything else

1. **Foundry invariant/fuzz suite for tbtc-v2** — still open as of
   2026-09-28, added alongside Hardhat
   (does not replace it — this is the standard dual-stack pattern:
   Hardhat for deployment/lifecycle tests, Foundry purely for fuzzing).
   Target the 4 highest-value invariants from the spec's §14 consolidated
   list first: claim≡anchor always, storage append-only/no collision
   across the `Bridge`↔`ReservationRouter` delegatecall boundary (already
   asserted example-by-example in `Bridge.RouterStorageParity.test.ts` —
   convert to a real fuzzed invariant), TBTC supply = live anchors + pooled
   backing, and no double-mint across the re-anchor settlement path (m1 B
   has no dissolution path to double-settle against, see §4). Cost: ~3-5
   days to stand up `forge` + write 4-6 invariant handlers reusing the
   existing fixture deployment logic.
2. **Add `-race` to CI for the packages reservation code touches**
   (`pkg/tbtc`, `pkg/chain/ethereum`, `pkg/bitcoin`) — still open as of
   2026-09-28; `client.yml`'s `gotestsum` invocations remain unflagged.
   Reservation coordination is concurrent (goroutine dispatch per wallet
   action, same pattern as existing heartbeat/redemption dispatch). It is
   free to add and catches races cheaply.
   Add `-race` **before** further coordination wiring lands, not after.
3. **Finish keep-core's own test bar — DONE (#4274, #4278, #4279).**
   `node_test.go`/`coordination_test.go` now have the same coverage every
   other action type has:
   `TestProcessCoordinationResult_ReservationAnchorRoutesToHandler`/
   `...ReanchorRoutesToHandler`,
   `TestCoordinationExecutor_GetActionsChecklist_Reservations`, and, going
   beyond the original ask, a full 3-operator leader/follower round-trip in
   `TestCoordinationExecutor_Coordinate_ReservationProposals`. See §2.
4. **Extend the existing fuzz-marshaling pattern — DONE (#4277).**
   `pkg/tbtc/marshaling_test.go` has
   `TestFuzzCoordinationMessage_MarshalingRoundtrip_WithReservationAnchorProposal`
   and `...WithReservationReanchorProposal`, mirroring every other proposal
   type verbatim. See §2.
5. **Run Gambit** (Certora's free, open-source Solidity mutation-testing
   tool — no Certora Prover license needed) against the reservation Mocha
   test suite — still open as of 2026-09-28. One-time run, tells you which
   assertions are illusory (execute the mutated line but don't actually
   fail) before an auditor finds the same gap manually. This is the
   single cheapest way to find out whether the current 12-file suite (§1)
   actually constitutes strong coverage or just line coverage.

### Tier 2 — moderate cost, still worth doing pre-audit

6. **Build the fork-based e2e test** — mainnet-fork Ethereum (the
   `FORKING_URL` config already exists, just unused for this feature) +
   Bitcoin regtest/testnet, exercising the full m1 B lifecycle: reserve ->
   accept -> settle -> re-anchor -> strand (dropped `renew`/`partial-redeem`/
   `dissolve` from the original walkthrough — out of m1 B scope, see §4).
   This isn't a new idea — it's explicitly required by the release
   runbook's own pre-audit checklist ("fork dry-run of the full activation
   sequence"), which is currently unchecked. Treat it as already-scoped
   work, not a new proposal. Still open as of 2026-09-28.
7. **Scale up the existing keep-core node/coordination harness to a
   multi-signer simulated integration test — partially done as of #4279.**
   `TestCoordinationExecutor_Coordinate_ReservationProposals` (see §2) is
   exactly this test for the anchor/reanchor proposal dispatch round-trip:
   N=3 in-process signers, real broadcast channel, real leader election,
   proving checklist generation -> leader election -> broadcast -> follower
   validation -> convergence for both proposal types. What it does **not**
   cover: a chained multi-action lifecycle through repeated coordination
   rounds (accept, then a later re-anchor, on the same reservation) — the
   existing test exercises each proposal type independently, not a
   sequence. Remaining gap: extend #4279's harness to run acceptance then
   re-anchor back-to-back on one reservation across two coordination
   windows.
8. **TLA+ model of the m1 B reservation protocol** (request -> authorize ->
   prove/settle -> re-anchor -> strand, with SPV-maintainer-stall and
   timeout transitions modeled explicitly; renew/dissolve/veto are
   M2-only — model them when m2 restores that surface, see §4). This is
   protocol-level, not code-level — it checks RFC 13's design for
   races/deadlocks across the contract/watchtower/wallet boundary that no
   single repo's test suite can see, because each side only ever tests its
   own half. Cheap relative to its payoff (a model-checker run finds the
   same class of bug a security review round found manually per the
   spec's §12 findings, but exhaustively rather than by inspection). Still
   open as of 2026-09-28.

### Tier 3 — stretch goals, run in parallel with the audit rather than gating it

9. **Narrow Certora Prover CVL specs** for the same ~4 invariants
   targeted by the Foundry suite in #1 (claim≡anchor, storage
   append-only, supply conservation, no double-mint across re-anchor).
   Proof-grade confidence instead of statistical confidence, but real cost (CVL
   learning curve + spec-writing, ~1-2 weeks) that isn't justified as a
   *blocking* step when Foundry fuzzing already gets most of the value
   for a quarter of the cost. Do this as a parallel track once the audit
   has started, not before.
10. **Property-based testing for keep-core's reservation state parsing**
    (e.g. via `pgregory.net/rapid`: any valid-per-spec sequence of state
    transitions never reaches an invalid `ReservationState`) — still lower
    priority than everything above, but its blocking premise is gone: the
    Go-side coordination/dispatch logic this would protect is wired now
    (Tier 1 item #3 is done, see §2). Re-rank alongside item 8 (TLA+) once
    Tier 2 is underway.

## 4. Sequencing relative to the merge/audit plan

**Reframed 2026-08-21 by the variant B decision** (`roadmap.md` §1). The
original routing below assumed Tier 1 tests would land inside the eight
tbtc-v2 stack PRs. m1 is now an essentials-only rewrite, so those PRs are
reference material and there is no stack merge to land tests inside
(`epic-merge-plan.md`, superseding note).

### Current routing

- **Tier 1 lands in the rewrite itself**, written alongside the m1 contracts
  rather than retrofitted into PRs. This is strictly better placement: the
  invariants exist before the code they constrain, and the fixtures are
  written once against the surface that actually ships.
- **Tier 2** remains the concrete content of the runbook's "fork dry-run"
  checklist item and is still already-required rather than optional.
- **Tier 3** still runs concurrently with the external audit so it adds no
  critical-path calendar time.

### What B changes about the test content

Scope corrections, not new tiers:

- **Drop dissolution/renewal/partial-redemption from every lifecycle
  walkthrough.** §0, Tier 1 item 1, and Tier 2 item 6 all originally ended
  their sequence at `dissolve` (Tier 2's cross-reference to "item 7" for
  this was itself wrong — item 7 is the multi-signer test, not the
  walkthrough; corrected to item 6 here), and Tier 2 item 8's TLA+ model
  included dissolution transitions. B has no dissolution, redemption, or
  renewal path, so the m1 lifecycle is reserve -> accept -> settle ->
  re-anchor -> strand. All of the above are now corrected directly at
  their own citations (this bullet documents why, not a pending action).
  Model dissolution/renewal/redemption when m2 restores them.
- **The old "no double-settle/double-mint across re-anchor + dissolution
  paths" invariant lost its dissolution half.** Re-anchor is the only
  settling action in m1 B (see the bullet above), so the invariant is now
  simply "no double-mint across re-anchor" — narrower but not less
  important, since re-anchor is also B's only unpin. Tier 1 item 1's
  citation above already reflects this.
- **Add the invariants B's launch gates imply**
  (`m1-b-implementation.md` §4): `activeReservationsCount` never exceeds
  `maxActiveReservations`; the cap itself stays below
  `liveWalletsCount x maxReservationsPerWallet`; acceptance always writes
  `dissolutionEligibleAt` even though nothing reads it; and no pause flag can
  block a settlement-path call, which is the property that keeps a confirmed
  Bitcoin spend settleable. (`ReservationVault.sol` is 373 lines total and
  has no pause/`Pausable` mechanism anywhere in it — the file cited here in
  earlier drafts, `:524-528`, is out of range; the substantive point holds,
  the citation didn't.)
- **Add a slot-exhaustion test.** B's characteristic failure is saturation, so
  drive occupancy to the cap and assert that acceptance reverts *before*
  re-anchor loses its last target — the whole point of §4.1's gate is that the
  cliff becomes a revert rather than a slashing event.

**Separately from B's scope correction:** §1-§2's file/function inventories
needed their own re-verification pass against the 2026-09-28 M1 tip (28
keep-core commits, several tbtc-v2 PRs, since the 2026-08-19 survey) — done
directly in §1-§2 above, not a B-scope issue.

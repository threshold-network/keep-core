# UTXO Reservations — Testing & Hardening Plan (pre-audit)

Status: DRAFT. Companion to `feature-spec.md` and
`epic-merge-plan.md`. Grounded in a direct inspection of both
repos' current tooling (2026-08-19), not assumptions.

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
  two-phase protocol itself** (request → authorize → prove/settle → renew
  → dissolve, with veto/timeout transitions), checked *before* finishing
  the keep-core wiring — it catches cross-system races between the
  contract, the watchtower, and the wallet that no amount of single-repo
  unit testing can see, because each repo's tests can only see its own
  side.

## 1. Current tooling — tbtc-v2 (Solidity), verified 2026-08-19

Checked out at `feat/utxo-reservation-partial-redemption` (stack tip).

- **Stack**: Hardhat + Mocha + Chai only. `solidity/package.json`
  devDependencies: `@nomiclabs/hardhat-waffle`, `chai`, `chai-as-promised`,
  `@types/mocha`. No Foundry, no Echidna, no Certora, no property-based
  testing library, no mutation testing tool.
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
- **Reservation-specific tests** (7 files, all pure Hardhat/Mocha/Chai,
  0 Foundry-style `invariant_`/`testFuzz_` functions found anywhere):
  - `solidity/test/bridge/ReservationRouter.test.ts` (~689 lines) —
    storage-layout parity across the delegatecall boundary.
  - `Reservation.test.ts`, `ReservationBacking.test.ts`,
    `ReservationGuards.test.ts`, `ReservationSettlement.test.ts`,
    `ReservationPartial.test.ts` — lifecycle, backing, guards, adversarial
    settlement, partial redemption.
  - `ReservationInvariants.test.ts` — named "invariants" but is standard
    Mocha `describe`/`it` asserting one example state each time, **not**
    property-based/fuzzed invariant checking. This is the single most
    misleading filename in the suite for an auditor skimming test names —
    worth relabeling or backing with real fuzzed invariants (§3.1) before
    review.
  - `solidity/test/deploy/95_deploy_reservation_vault.test.ts` — deploy
    script test.

## 2. Current tooling — keep-core (Go), verified 2026-08-19

Checked out at `feat/utxo-reservation-wallet-support`.

- **Stack**: `testify` (indirect dep only), `google/gofuzz` (indirect).
  No ginkgo/gomega, gomock, gopter, or `pgregory.net/rapid`.
- **Native Go fuzzing**: real `func Fuzz*` functions exist only in
  `pkg/internal/pbutils/pbutils.go` (`FuzzUnmarshaler`, `FuzzFuncs`,
  `fuzzBigInt`, `fuzzEphemeralPublicKey/PrivateKey`, `fuzzG1`/`fuzzG2`) —
  this is a real, working fuzz harness the project already trusts, and
  every existing wallet-action proposal type calls it, e.g.
  `TestFuzzCoordinationMessage_Unmarshaler` in `pkg/tbtc/marshaling_test.go`
  runs `pbutils.FuzzUnmarshaler(&coordinationMessage{})`, plus dedicated
  `TestFuzzCoordinationMessage_MarshalingRoundtrip_With*Proposal` loops (10
  iterations each) for Heartbeat/DepositSweep/Redemption/MovingFunds/
  MovedFundsSweep/Noop proposals. **Reservation proposals have no
  equivalent entry** — confirmed by grep, and expected, since the
  protobuf reservation message types don't exist yet either (JSON
  placeholder marshaling only, per the PR's own TODOs).
- **CI** (`.github/workflows/client.yml`): `client-build-test-publish` runs
  `gotestsum -- -timeout 15m -coverprofile=... ./...` (no `-race` flag);
  `client-integration-test` runs `gotestsum -- -timeout 20m
  -tags=integration ./...` but is gated to non-PR events only. Exactly 2
  files repo-wide use the `//go:build integration` tag
  (`pkg/bitcoin/electrum/electrum_integration_test.go`,
  `pkg/chain/ethereum/ethereum_integration_test.go`) — neither touches
  reservations.
- **Reservation-specific tests** — more substantial than a first pass
  suggested; direct read of `pkg/tbtc/reservation_test.go` (770 lines, 9
  functions) found real transaction-assembly-level tests mirroring the
  existing action convention: `TestAssembleReservedRedemptionTransaction`,
  `TestAssembleReservationDissolutionTransaction`,
  `TestReservationProposals_MarshalingRoundtrip`,
  `TestReservationProposals_UnmarshalRejectsMissingIntegers`,
  `TestAssembleReservationTransactions_InputValidation`, plus enum/state
  parse tests. These use `newLocalBitcoinChain()`, the same harness pattern
  as `TestAssembleRedemptionTransaction`/`TestAssembleDepositSweepTransaction`.
  Other touched files: `pkg/chain/ethereum/tbtc_test.go` (error-path tests
  against the 5 stubbed Ethereum methods — they assert `"not supported
  yet"`, nothing functional), `pkg/clientinfo/performance_test.go` (adds
  reservation action types to an expected-metrics list).
- **The real gap**: `coordination_test.go` and `node_test.go` — the tests
  that exercise the leader/follower coordination routine and the
  proposal-to-handler dispatch (`TestCoordinationExecutor_Coordinate`,
  `TestNode_HandleRedemptionProposal_DispatchesAction`,
  `TestProcessCoordinationResult_RedemptionRoutesToHandler`, etc. — every
  existing action type has these) — **contain zero references to
  `Reservation`**, confirmed by direct grep. Transaction assembly is
  tested; the coordination/dispatch integration that would make a wallet
  actually *execute* a reservation action end to end is not, because it
  isn't wired yet (matches the earlier gap analysis: no
  `handleReservationProposal`, no checklist entry, Ethereum bindings
  stubbed with errors).

## 3. Recommendations, ranked by bang-for-buck

### Tier 1 — cheap, high value, do before anything else

1. **Foundry invariant/fuzz suite for tbtc-v2**, added alongside Hardhat
   (does not replace it — this is the standard dual-stack pattern:
   Hardhat for deployment/lifecycle tests, Foundry purely for fuzzing).
   Target the 4 highest-value invariants from the spec's §14 consolidated
   list first: claim≡anchor always, storage append-only/no collision
   across the `Bridge`↔`ReservationRouter` delegatecall boundary, TBTC
   supply = live anchors + pooled backing (already asserted example-by-
   example in `ReservationInvariants.test.ts` — convert to a real fuzzed
   invariant), and no double-settle/double-mint across re-anchor +
   dissolution paths. Cost: ~3-5 days to stand up `forge` + write 4-6
   invariant handlers reusing the existing fixture deployment logic.
2. **Add `-race` to CI for the packages reservation code touches**
   (`pkg/tbtc`, `pkg/chain/ethereum`, `pkg/bitcoin`) — currently absent
   repo-wide. Reservation coordination is concurrent (goroutine dispatch
   per wallet action, same pattern as existing heartbeat/redemption
   dispatch). Free to add, catches races cheaply, should happen before
   the coordination wiring lands, not after.
3. **Finish keep-core's own test bar, don't treat it as new tooling**:
   extend `node_test.go`/`coordination_test.go` with the same coverage
   every other action type already has —
   `TestNode_HandleReservationProposal_*` (uncontrolled wallet, wallet
   busy, dispatches action), `TestProcessCoordinationResult_ReservationRoutesToHandler`,
   inclusion in `TestCoordinationExecutor_GetActionsChecklist`. This isn't
   optional hardening, it's the same convention the codebase already
   holds every other wallet action to — its absence is a completeness gap,
   not a stylistic choice.
4. **Extend the existing fuzz-marshaling pattern** — once reservation
   protobuf types exist, add
   `TestFuzzCoordinationMessage_MarshalingRoundtrip_WithReservationProposal`
   and a `pbutils.FuzzUnmarshaler(&reservationProposal{})` call, mirroring
   every other proposal type verbatim. Near-zero cost; put it on the
   checklist for whoever does the coordination-wiring PR so it isn't
   forgotten.
5. **Run Gambit** (Certora's free, open-source Solidity mutation-testing
   tool — no Certora Prover license needed) against the existing ~2,500+
   lines of reservation Mocha tests. One-time run, tells you which
   assertions are illusory (execute the mutated line but don't actually
   fail) before an auditor finds the same gap manually. This is the
   single cheapest way to find out whether "7 test files, hundreds of
   `it` blocks" actually constitutes strong coverage or just line
   coverage.

### Tier 2 — moderate cost, still worth doing pre-audit

6. **Build the fork-based e2e test** — mainnet-fork Ethereum (the
   `FORKING_URL` config already exists, just unused for this feature) +
   Bitcoin regtest/testnet, exercising the full lifecycle from the spec's
   walkthrough (reserve → accept → settle → renew → partial-redeem →
   dissolve/strand). This isn't a new idea — it's explicitly required by
   the release runbook's own pre-audit checklist ("fork dry-run of the
   full activation sequence"), which is currently unchecked. Treat it as
   already-scoped work, not a new proposal.
7. **Scale up the existing keep-core node/coordination harness to a
   multi-signer simulated integration test** — N=`GroupSize` in-process
   signers via the existing `localChain` + node-harness pattern, running
   a full reservation lifecycle through real coordination rounds (not
   single-node dispatch tests). This is the keep-core equivalent of #6 —
   currently nothing exercises reservation actions past a single node.
8. **TLA+ model of the two-phase reservation protocol** (request →
   authorize → prove/settle → renew/expire → dissolve, with watchtower
   veto and timeout transitions modeled explicitly). This is protocol-
   level, not code-level — it checks RFC 13's design for races/deadlocks
   across the contract/watchtower/wallet boundary that no single repo's
   test suite can see, because each side only ever tests its own half.
   Cheap relative to its payoff (a model-checker run finds the same class
   of bug a security review round found manually per the spec's §12
   findings, but exhaustively rather than by inspection).

### Tier 3 — stretch goals, run in parallel with the audit rather than gating it

9. **Narrow Certora Prover CVL specs** for the same ~4 invariants
   targeted by the Foundry suite in #1 (claim≡anchor, storage
   append-only, supply conservation, no double-settle). Proof-grade
   confidence instead of statistical confidence, but real cost (CVL
   learning curve + spec-writing, ~1-2 weeks) that isn't justified as a
   *blocking* step when Foundry fuzzing already gets most of the value
   for a quarter of the cost. Do this as a parallel track once the audit
   has started, not before.
10. **Property-based testing for keep-core's reservation state parsing**
    (e.g. via `pgregory.net/rapid`: any valid-per-spec sequence of state
    transitions never reaches an invalid `ReservationState`) — lower
    priority than everything above because the Go-side coordination logic
    this would protect isn't wired yet (Tier 1 item #3 comes first).

## 4. Sequencing relative to the merge/audit plan

Tier 1 items should land *inside* the existing tbtc-v2 stack PRs or as a
follow-up PR merged into `reservations-epic` before the stack merges to
`main` (§3 of `epic-merge-plan.md`) — they're cheap enough
that there's no reason to let them slip past the audit gate. Tier 2 items
are the concrete content of the runbook's "fork dry-run" checklist item
(§5 of the merge plan) and should be treated as already-required, not
optional. Tier 3 can run concurrently with the external audit engagement
so it doesn't add calendar time on the critical path.

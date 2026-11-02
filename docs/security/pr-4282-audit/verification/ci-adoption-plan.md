# Plan: bring the reservation verification tooling into CI

Status: decided on 2026-10-12. Step 1 runs on the private tooling line; steps 2-4 are planned.

## Current state

| Tool | Where it lives now | Runs in CI today | What CI needs |
|---|---|---|---|
| 10 native Go fuzz targets | keep-core, a tooling branch (not merged) | No | The seed inputs run as ordinary tests in `go test ./...` as soon as the files are merged. Coverage-guided fuzzing needs ClusterFuzzLite: add 10 `compile_native_go_fuzzer` lines to `.clusterfuzzlite/build.sh`. Its target-registration check fails if they are missing. |
| Invariant suite (Foundry part) | tbtc-v2, a tooling branch (not merged) | No | Nothing extra. The `contracts-foundry` job on `origin/dev` runs `yarn test:foundry` with `FOUNDRY_PROFILE=ci`, which runs this suite (256 runs of 100 calls, about 40 s). The PR branch `reservations-upgrade` does not have this job until it is rebased onto `dev`. |
| Invariant suite (Medusa part) | same | No | A new workflow, modelled on the existing `contracts-fizz.yml` pattern with a known-failures list. |
| Hand-off liveness property | same | No, by design | Expected to fail until the saturation design is decided. Keep it out of CI. |
| Solidity coverage | tbtc-v2, a tooling branch (not merged) | No | A job that runs `yarn coverage`. The full run takes several minutes and 18 gas-assertion tests fail under instrumentation, so the job cannot be blocking. |
| Mutation testing | none (report only) | No | A run takes 3.5 to 5.5 hours. Not suitable for per-PR CI. |
| New regression tests | the reservation review branch (keep-core), the integrated reservation branch (tbtc-v2) | Yes, once merged | Nothing. They are ordinary unit tests in existing suites. |

## Exposure

All items are test or tooling changes for reservation code, which does not run on mainnet today. Under the
split-by-exposure rule they may go to `origin` after the human approves. D24 sets the time: after the drill.

## Proposed order

1. Now, on the private tooling line only:
   1. Run the 10 Go targets nightly with native Go fuzzing. Register them in ClusterFuzzLite once the
      reservation packages reach the tooling line's `dev`.
   2. Run the Medusa accounting config nightly for 30 minutes; a property failure opens an issue.
   3. Run coverage nightly as a non-blocking job.
2. After the drill, with the human's approval:
   1. Add the Go fuzz targets and the matcher test to PR #4282.
   2. Rebase PR #1116 onto `dev` so it gets the `contracts-foundry` job. Then add the invariant suite
      (without the liveness files), the coverage tooling, the vault test and the `Bridge.Parameters` fixture
      fix.
3. After the saturation design decision:
   1. Turn the liveness property into a passing invariant, or keep it as a documented exploratory check.
   2. Move it into the Foundry run if it passes.
4. Before mainnet: run the mutation pilot again on the release candidate, as a report, not a gate.

## Decisions (human, 2026-10-12)

- Private nightly Medusa job: approved.
- Coverage: a non-blocking nightly job, approved.
- ClusterFuzzLite registration: approved, but not possible yet. ClusterFuzzLite lives on the private `dev`
  line, which has no reservation packages, so its build would fail. Until the reservation code reaches `dev`,
  a nightly native Go fuzz job runs the 10 targets instead.
- Go public with the test changes after the drill: approved.
- Where the nightly jobs live: on the private tooling line's default branch, checking out the reservation
  branches by name. Scheduled workflows run only from the default branch, and neither private `main` nor
  `dev` carries the reservation code.

## Done

- tbtc-v2, private tooling line: nightly Medusa (30 minutes, files an issue on a property failure) and
  non-blocking coverage jobs.
- keep-core, private tooling line: nightly native Go fuzzing, one matrix job per target, files an issue on a
  failure.
- Both pass `actionlint`. The Medusa binary is pinned by URL and SHA-256. The issue steps run only when the
  fuzz step itself fails. The keep-core job passes the private `verification-gate.sh --check-body` parser.
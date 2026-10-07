# Bundled random-beacon deploy scripts

This directory contains a committed copy of the `export/deploy/*.js` scripts that
the `@keep-network/random-beacon` package publishes to npm. The ecdsa package
needs these so its hardhat-deploy run can resolve random-beacon's deploy phase
when the local `../random-beacon/export/` directory is unavailable (it is
gitignored) and falling back to `node_modules/@keep-network/random-beacon/export`
would pull a stale published version.

The resolution order is defined in `solidity/ecdsa/hardhat.config.ts`
(`resolveRandomBeaconExport`): local sibling export first, this bundled copy
second, npm fallback last.

This README lives one level above `deploy/` because hardhat-deploy walks that
directory and tries to `require()` every file; a Markdown sibling there would
crash deployment.

## Source

The scripts are the TypeScript-compiled output of `solidity/random-beacon/deploy/*.ts`,
produced by `yarn prepack` (i.e. `tsc -p tsconfig.export.json`) in the
`@keep-network/random-beacon` package.

## Format

Every bundled script is `tsc`-compiled ES5 output from the upstream package's
TypeScript sources (`__awaiter` / `__generator` runtime helpers, `var`
declarations). Treat them as build artifacts; do not hand-edit.

`05_approve_random_beacon_in_token_staking.js` checks that `TokenStaking` has
`approveApplication` before it calls it, so it skips cleanly on the Threshold
`TokenStaking` ABI, which does not expose it. It also skips an application whose
status is not `NOT_APPROVED`. Reading `applicationInfo(...)` is the only call
whose errors it swallows; a revert from `approveApplication(...)` propagates.

## Verification

Explorer verification in these scripts calls `helpers.etherscan.verify(...)` and
`hre.tenderly.verify(...)` directly, without the ecdsa package's
`verifyOn...OrContinue` wrappers. A Tenderly failure can therefore halt the
deploy. The Etherscan helper logs its errors instead of throwing. Script 04
verifies its contracts on every run, so a failed attempt is retried by a later
run.

## Regenerate

From the repo root:

```sh
cd solidity/random-beacon
yarn install
yarn prepack
cp export/deploy/0*.js ../ecdsa/external/random-beacon-export/deploy/
```

Then verify `git diff` matches the intended deploy-script change in the
sibling `solidity/random-beacon/deploy/*.ts` source — divergence between
the `.ts` source and the bundled `.js` is the failure mode this directory
guards against.

### Regeneration policy

Regenerate every script whenever `solidity/random-beacon/deploy/*.ts` changes,
and commit the result with that change. Then run the ecdsa tests against a network
that exposes `approveApplication` (legacy Keep TokenStaking) and one that does
not (Threshold TokenStaking).

## Why we don't just `ts-node` the upstream

`hardhat-deploy` reads deploy scripts from the configured external paths as
plain CommonJS modules. The `external/*/deploy` directories are listed in
`hardhat.config.ts` and loaded via `require`, so they must be runnable JS.
The bundled `.js` here matches what `@keep-network/random-beacon` ships to
npm consumers, keeping the in-monorepo and published-consumer code paths
identical.

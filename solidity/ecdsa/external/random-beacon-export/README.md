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

The bundled scripts intentionally mix two formats:

- **`01..04, 06..09_*.js`**: `tsc`-compiled ES5 output from the upstream
  package's TypeScript sources (`__awaiter` / `__generator` runtime helpers,
  `var` declarations). Treat as build artifacts; do not hand-edit.
- **`05_approve_random_beacon_in_token_staking.js`**: hand-written modern
  async/await. Adds an `ifaceHasFunction("approveApplication")` precheck (so it
  skips cleanly on the Threshold `TokenStaking` ABI, which does not expose
  `approveApplication`) plus an idempotency guard that swallows errors only
  while reading `applicationInfo(...)`. The `approveApplication(...)` call
  itself is intentionally left unwrapped so a genuine revert propagates.
  **Do not regenerate from upstream without preserving this precheck** —
  blind regeneration will reintroduce a hard failure on networks running the
  Threshold staking contract.

## Known limitation: verification is not wrapped

Unlike the hand-maintained ECDSA deploy scripts (which route Etherscan/Tenderly
verification through `verifyOnEtherscanOrContinue` / `verifyOnTenderlyOrContinue`
so explorer outages never abort a deploy), the `tsc`-compiled vendored scripts
call `helpers.etherscan.verify(...)` / `hre.tenderly.verify(...)` directly. A
verification failure (rate limit, bytecode mismatch, missing key) in one of
these scripts can therefore halt the deploy.

This is accepted rather than patched: these are build artifacts and must not be
hand-edited (see Format above). If it becomes a recurring operational problem,
fix it upstream in `@keep-network/random-beacon`'s `export/deploy` sources and
re-vendor, or set `DISABLE_HARDHAT_VERIFY` / the network's verify tags off for
the run.

## Regenerate

From the repo root:

```sh
cd solidity/random-beacon
yarn install
yarn prepack
# Copy every script EXCEPT 05_* — that one is hand-maintained (see below).
cp export/deploy/0[1-4]_*.js ../ecdsa/external/random-beacon-export/deploy/
cp export/deploy/0[6-9]_*.js ../ecdsa/external/random-beacon-export/deploy/
```

Then verify `git diff` matches the intended deploy-script change in the
sibling `solidity/random-beacon/deploy/*.ts` source — divergence between
the `.ts` source and the bundled `.js` is the failure mode this directory
guards against.

### Regeneration policy

When syncing from upstream:

1. Regenerate `01..04, 06..09_*.js` from `@keep-network/random-beacon`'s
   `export/deploy` source via its `tsc` build (the `yarn prepack` step above).
2. **Skip `05_*.js`** during bulk regeneration — it is maintained deliberately.
   If you do regenerate it, ensure it matches
   `solidity/random-beacon/deploy/05_approve_random_beacon_in_token_staking.ts`
   and preserves the `ifaceHasFunction("approveApplication")` gating and the
   `applicationInfo(...)` idempotency check.
3. Verify by running deploys against both a network that exposes
   `approveApplication` (legacy Keep TokenStaking) and one that does not
   (Threshold TokenStaking).

## Why we don't just `ts-node` the upstream

`hardhat-deploy` reads deploy scripts from the configured external paths as
plain CommonJS modules. The `external/*/deploy` directories are listed in
`hardhat.config.ts` and loaded via `require`, so they must be runnable JS.
The bundled `.js` here matches what `@keep-network/random-beacon` ships to
npm consumers, keeping the in-monorepo and published-consumer code paths
identical.

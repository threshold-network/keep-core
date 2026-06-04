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

## Source

The scripts are the TypeScript-compiled output of `solidity/random-beacon/deploy/*.ts`,
produced by `yarn prepack` (i.e. `tsc -p tsconfig.export.json`) in the
`@keep-network/random-beacon` package.

## Regenerate

From the repo root:

```sh
cd solidity/random-beacon
yarn install
yarn prepack
cp export/deploy/*.js ../ecdsa/external/random-beacon-export/deploy/
```

Then verify `git diff` matches the intended deploy-script change in the
sibling `solidity/random-beacon/deploy/*.ts` source — divergence between
the `.ts` source and the bundled `.js` is the failure mode this directory
guards against.

## Why we don't just `ts-node` the upstream

`hardhat-deploy` reads deploy scripts from the configured external paths as
plain CommonJS modules. The `external/*/deploy` directories are listed in
`hardhat.config.ts` and loaded via `require`, so they must be runnable JS.
The bundled `.js` here matches what `@keep-network/random-beacon` ships to
npm consumers, keeping the in-monorepo and published-consumer code paths
identical.

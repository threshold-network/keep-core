# Frozen Beacon deployment compatibility

ECDSA uses this committed snapshot for legacy deployment replay. It no longer
installs the published Random Beacon package, which pulled sortition-pools back
into the dependency graph after its direct removal.

`artifacts/` contains the eleven required deployment artifacts from
`@keep-network/random-beacon@2.1.0-dev.18`. ABI, bytecode, link references,
metadata (including original Solidity sources), storage layouts, and docs are
preserved. Duplicate EVM assembly/opcode output is omitted. `VENDOR.json` records
provenance and upstream hashes. The embedded SortitionPool artifact retains
the upstream ISC licensing (see `../../contracts/legacy/sortition/LICENSE`). The deployment scripts below retain their
existing fixes. Contract interfaces and reimbursement support live in
`contracts/legacy/random-beacon`; initialization tasks live in
`tasks/legacy-random-beacon` and are copied to `export/tasks` during packaging.

There is no npm fallback or automatic refresh from a dist-tag. Updates to this
legacy snapshot must be explicit, reviewed changes. Keep this README outside
`deploy/`, because hardhat-deploy requires every file in that directory.

## Source

Except for the hand-maintained `05_approve_random_beacon_in_token_staking.js`
(see Format below), the scripts are the TypeScript-compiled output of
`solidity/random-beacon/deploy/*.ts`, produced by `yarn prepack` (i.e.
`tsc -p tsconfig.export.json`) in the `@keep-network/random-beacon` package.

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

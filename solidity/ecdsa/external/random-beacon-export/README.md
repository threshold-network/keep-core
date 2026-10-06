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

The scripts are the TypeScript-compiled output of
`solidity/random-beacon/deploy/*.ts`, produced by `yarn prepack` (i.e.
`tsc -p tsconfig.export.json`) in the `@keep-network/random-beacon` package.

## Format

The committed scripts are `tsc`-compiled ES5 output from the upstream
package's TypeScript sources (`__awaiter` / `__generator` runtime helpers,
`var` declarations). Treat as build artifacts; do not hand-edit. The approval
script's missing-function and already-approved guards now live in the
TypeScript source on `dev`, so regeneration is uniform across all nine scripts
and produces ES2020/CommonJS output with ethers v6. The frozen ES5 scripts are
the committed state; regeneration from the current source produces ES2020
output that supersedes them once re-verified.

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

From `solidity/random-beacon`, regenerate with:

```sh
yarn prepack
cp export/deploy/*.js ../ecdsa/external/random-beacon-export/deploy/
mkdir -p ../ecdsa/external/random-beacon-export/utils
cp export/utils/wait-for-confirmations.js ../ecdsa/external/random-beacon-export/utils/
mkdir -p ../ecdsa/external/random-beacon-export/tasks/utils
cp export/tasks/initialize.js export/tasks/unlock-eth-accounts.js ../ecdsa/external/random-beacon-export/tasks/
cp export/tasks/utils/*.js ../ecdsa/external/random-beacon-export/tasks/utils/
```

Compare the generated files with their sources, then exercise ECDSA with the
sibling export unavailable. Separately test actual npm tarballs with
`RANDOM_BEACON_EXPORT_PATH`; a passing bundled fallback does not validate a
published producer. See [the compatibility checks](../../../docs/ethers-v6-compatibility.md).

The Beacon scripts still call explorer verification directly when the network
tags enable it. Verification failure can halt a deploy. The local compatibility
checks do not test explorer services or enable public-network tags.

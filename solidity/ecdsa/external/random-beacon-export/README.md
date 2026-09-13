# Bundled Random Beacon executable exports

ECDSA uses these compiled exports when the sibling Beacon build is absent. The
normal resolution order for deployment scripts and tasks is sibling `export/`,
this bundle, then the pinned npm package. Artifacts use the sibling build or npm.
`RANDOM_BEACON_EXPORT_PATH` explicitly selects a producer export root and fails
if a requested `deploy/`, `artifacts/` or `tasks/` directory is missing; package
compatibility checks use it to prevent fallback from hiding omissions.

ECDSA's initialization, authorization, registration and account-unlock tasks use
the same resolver. This bundle includes the v6 Beacon initialization and unlock
tasks plus their utilities, so the pinned v5 package supplies no executable task
code. Packed ECDSA exports include this bundle and prefer it over the installed
Beacon dependency until that dependency is migrated.

All nine deployment scripts come from `solidity/random-beacon/deploy/*.ts`, compiled as
ES2020/CommonJS with ethers v6. The approval script's missing-function and
already-approved guards live in the TypeScript source, so it is regenerated with
the other scripts. `utils/wait-for-confirmations.js` is also required by the
explorer-tagged deployment paths. Do not hand-edit generated JavaScript.

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

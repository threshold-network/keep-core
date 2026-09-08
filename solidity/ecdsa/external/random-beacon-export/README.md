# Bundled Random Beacon deploy scripts

ECDSA uses these compiled exports when the sibling Beacon build is absent. The
normal resolution order is sibling `export/`, this bundled deploy directory,
then the pinned npm package. `RANDOM_BEACON_EXPORT_PATH` explicitly selects a
producer export root and fails if either `deploy/` or `artifacts/` is missing;
package compatibility checks use it to prevent fallback from hiding omissions.

All nine scripts now come from `solidity/random-beacon/deploy/*.ts`, compiled as
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
```

Compare the generated files with their sources, then exercise ECDSA with the
sibling export unavailable. Separately test actual npm tarballs with
`RANDOM_BEACON_EXPORT_PATH`; a passing bundled fallback does not validate a
published producer. See [the compatibility checks](../../../docs/ethers-v6-compatibility.md).

The Beacon scripts still call explorer verification directly when the network
tags enable it. Verification failure can halt a deploy. The local compatibility
checks do not test explorer services or enable public-network tags.

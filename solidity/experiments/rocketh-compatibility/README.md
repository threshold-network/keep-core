# Rocketh deployment compatibility experiment

This isolated fixture supports [keep-core #4295](https://github.com/threshold-network/keep-core/issues/4295).
It tests the proposed Rocketh deployment / ethers v6 test boundary using packed
producer scripts and two in-process Hardhat networks. It does not change either
production package's toolchain.

Tested with Node 24.11.1 and npm 11.6.2. Use the checked-in lockfiles:

```sh
cd solidity/experiments/rocketh-compatibility
npm ci --ignore-scripts --no-audit --no-fund
npm ci --prefix baseline --ignore-scripts --no-audit --no-fund
npm test
```

The test run uses a local, pinned solc-js compiler, packs and installs the producer
without publishing it, and runs all deployments on in-process simulated networks.
The temporary producer installation is not saved to package.json or the lockfile.
It requires the installed dependencies and their npm cache; the temporary package
installation runs offline. Generated files stay under ignored directories.

The runner first compiles the probe contracts. It gives the same compiled
artifacts to a separate Hardhat 2 / hardhat-deploy v1 baseline, then compares
that engine's actual deployment/export results with Hardhat 3 / Rocketh.

The 10 checks cover:

- Loading scripts and their artifacts from an npm tarball.
- Cross-package tag dependencies and consumer-provided accounts.
- Idempotent reruns and ethers v6 snapshot fixtures.
- Equal deployment addresses and ABIs across the two engines.
- Byte-identical single-network v1 JSON from a prototype adapter, including
  linkedData and checksummed contract addresses.
- The different native Rocketh JSON schema.
- Two explicitly constructed Contracts 4 proxies sharing a ProxyAdmin.
- Rejection of an incompatible storage layout, a successful existing-proxy upgrade,
  ownership transfer, and replacement of one proxy's admin.

The proxy setup uses validated implementation deployment and forceImport through
OpenZeppelin's Hardhat 3 API. It is an example of preserving legacy behavior, not
a selected production adapter. No validation bypass is used.

Inspect `.run/v1-export.json`, `.run/v2-legacy-export.json`,
`.run/v2-native-export.json`, `.run/v2-records.json`, and
`.run/packed-files.json` after a successful run.

This does not establish compatibility for the real upstream scripts,
`export/artifacts`, compiler output, verification integrations, production
deployment records/manifests, or WalletRegistry's linked libraries and full
proxy/deputy workflow. See [the decision record](../../docs/rocketh-feasibility.md)
for the implementation gates.

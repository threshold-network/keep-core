# Hardhat 3 and deployment export migration

Status: migration preparation, 2026-09-07. Track the runtime migration in
[#4209](https://github.com/threshold-network/keep-core/issues/4209), the remaining
plugin namespace changes in
[#4213](https://github.com/threshold-network/keep-core/issues/4213), and the
cross-package deployment conversion in
[#4295](https://github.com/threshold-network/keep-core/issues/4295).

## Completed preparation

Both packages can use Hardhat Chai Matchers on ethers v5. Random Beacon now uses
Hardhat Network Helpers for its snapshot fixtures and `ethers.provider` for
provider access, removing its Waffle dependencies. Existing deployment fixtures
still use hardhat-deploy v1 APIs. Verification uses
`@nomicfoundation/hardhat-verify` in both packages.

The preceding stack updates TypeChain, enables strict TypeScript, raises the
published JavaScript target to ES2020 while retaining CommonJS, updates runtime
types, and modernizes lint/format tooling. These changes do not convert deployment
scripts to Rocketh. The independent maintained-v1 update is
[#4294](https://github.com/threshold-network/keep-core/pull/4294).

## Public deployment API

The packages execute upstream deployment scripts from published npm packages:

```text
@threshold-network/solidity-contracts -> random-beacon -> ecdsa -> tbtc-v2
```

`hardhat.config.ts` in each package loads
`@threshold-network/solidity-contracts/export/deploy`. ECDSA additionally loads
Random Beacon's `export/deploy` through `resolveRandomBeaconExport`. Downstream
tbtc-v2 executes ECDSA's published exports. Therefore the module format and script
entry points in `export/deploy` are a public API, alongside `export/artifacts`,
`export.json`, and the committed `deployments/{mainnet,sepolia}` records.

Raising the ES target does not change this CommonJS API. The ES2020 migration was
checked against ES5 using fresh local deployments: the exported JSON was identical
for all 16 beacon and 22 ECDSA contracts. A switch to ESM/Rocketh needs a separate
consumer compatibility check.

## Proposed direction and remaining blockers

Use ethers v6 for tests with viem confined to the Rocketh deployment layer. This
matches the direction recorded in
[tbtc-v2 #1128](https://github.com/threshold-network/tbtc-v2/issues/1128).
Prefer dual publication during the transition: retain the current CommonJS
`export/deploy` entry point and introduce a separate, explicitly versioned ESM
entry point. These are implementation proposals; the export path, shared source
strategy, and release sequence still need agreement with downstream maintainers.
Do not replace the existing export in place before that agreement.

The following gates remain:

| Gate                           | Evidence and required next step                                                                                                                                                                                                                                                   |
| ------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| ethers v6                      | Both packages still use ethers v5 and `@nomiclabs/hardhat-ethers` 2.x. Port BigNumber arithmetic, provider/contract APIs, TypeChain target, and tests before replacing this plugin.                                                                                               |
| Chai / Mocha / Hardhat         | Migrate together to the compatible Hardhat 3 Mocha/ethers toolchain after ethers v6. The current Chai 4/Mocha 10 type declarations intentionally match the Hardhat 2 runtime.                                                                                                     |
| hardhat-deploy v2              | Published `hardhat-deploy@2.0.26` requires Hardhat `^3.6.0`, Rocketh `^0.21.0`, and `@rocketh/node ^0.21.0`. It is a deployment API conversion, not a drop-in version bump.                                                                                                       |
| Shared helpers                 | The installed `@keep-network/hardhat-helpers@0.6.0-pre.15` peers with hardhat-deploy `~0.11.11`. The published `0.7.2` line still peers with Hardhat `^2.19.4` and deploy `^0.11.45`; it does not provide a Hardhat 3 migration path. Port or replace the used helper APIs first. |
| Other plugins                  | Beacon's installed OpenZeppelin Upgrades 1.20.0, Tenderly 1.0.12, and TypeChain Hardhat 7.0.0 plugins all declare Hardhat 2 peers. Inventory both packages' integrations and select compatible versions or replacements before the cutover.                                       |
| Upstream deployment scripts    | Both packages consume `@threshold-network/solidity-contracts@1.3.0-dev.14`. Its current export supplies v1 scripts. Obtain compatible v2 exports before converting dependent deployment paths.                                                                                    |
| Cross-package script discovery | The v2 migration guide does not document a replacement for `external.contracts[].deploy`. Establish and test how one package executes another package's scripts before selecting the new export layout.                                                                           |
| Artifact/export compatibility  | `@rocketh/export` exists, but equivalence with the current `--export export.json` and `hardhat export-artifacts` output has not been established. Compare actual output and account for every consumer before changing it.                                                        |

The v2 package includes a CommonJS compatibility entry, but that does not make the
Rocketh deployment API or the existing v1 scripts interchangeable. The upstream
[migration guide](https://github.com/wighawag/rocketh/blob/main/hardhat-deploy/documentation/how-to/migration-from-v1/index.md)
describes ESM scripts, Rocketh configuration and extensions, and replacement
fixture loaders. Version evidence is available in the published metadata for
[hardhat-deploy 2.0.26](https://registry.npmjs.org/hardhat-deploy/2.0.26) and
[hardhat-helpers 0.7.2](https://registry.npmjs.org/@keep-network%2fhardhat-helpers/0.7.2).

## Cutover acceptance checks

1. Agree and test the cross-package ESM entry point and dual-publishing strategy
   with the upstream contracts package and tbtc-v2. Keep the current CJS exports
   available until the last v1 consumer migrates.
2. Land the ethers v6 and helper migrations with full test coverage, then introduce
   the compatible Chai/Mocha/Hardhat 3 and Rocketh toolchain together.
3. Convert and publish upstream contracts first, then Random Beacon's 9 deploy
   scripts, then ECDSA's 22 deploy scripts. Convert the deployment fixture loaders
   and external-script integration alongside each package's scripts.
4. Run both full suites and fresh local deployments that exercise the external
   scripts. Compare addresses, ABIs, start blocks, exported artifacts, and committed
   network deployment records against the v1 baseline; review and coordinate every
   intentional format difference.
5. Validate the published package tarballs in ECDSA and tbtc-v2, including the old
   CJS consumer path and new ESM path during dual publication. Retire the CJS path
   only in an agreed breaking release.

Keep #4209, #4213, and #4295 open until their respective migrations are complete.

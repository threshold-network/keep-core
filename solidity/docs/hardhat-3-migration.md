# Hardhat 3 and deployment export migration

Status: ethers v6 preparation on Hardhat 2, 2026-09-08. Track the runtime migration
in [#4209](https://github.com/threshold-network/keep-core/issues/4209), plugin
changes in [#4213](https://github.com/threshold-network/keep-core/issues/4213), and
cross-package deployment conversion in
[#4295](https://github.com/threshold-network/keep-core/issues/4295).

## Decision

Target Hardhat 3, hardhat-deploy v2 and Rocketh, with viem in the deployment layer
and ethers v6 plus Mocha/Chai for tests. Keep Hardhat 2 and maintained
hardhat-deploy v1 available while consumers migrate. Prefer separate, versioned
release lines for the CommonJS/v1 and ESM/Rocketh executable APIs. Same-package
dual publication is possible, but adds a second runtime contract to every
producer and remains an alternative requiring consumer agreement.

The dependency order is:

```text
@threshold-network/solidity-contracts -> random-beacon -> ecdsa -> tbtc-v2
```

Consumers execute the producers' `export/deploy` scripts. Module format, runtime
plugins and imported support files are therefore public APIs. Treat them
separately from the data APIs: `export/artifacts`, `export.json`, and committed
network deployment records. CommonJS alone does not make an ethers v6 script
compatible with an ethers v5 consumer.

## Preparation in this stack

Both packages use ethers v6, its Hardhat Chai Matchers integration and TypeChain
generator, while retaining Hardhat 2.29.0, maintained hardhat-deploy 1.0.4 and
ES2020/CommonJS exports. Waffle is removed. Strict TypeScript remains enabled.
Tests, tasks, deployment scripts and mock helpers use bigint and ethers v6 APIs.

The helper moves to 0.7.2, which already uses ethers v6. Its declared deployment
and upgrade peers still need compatibility patches: this stack retains
OpenZeppelin Upgrades 2.5.1 and the shared OpenZeppelin v4 ProxyAdmin behavior.
Other patches preserve proxy receipt fields, support the verification plugin's
Etherscan v2 API, handle a TypeChain reserved name and adapt the three ethers v5
API uses in the pinned Threshold deployment export. See the
[checks and patch inventory](ethers-v6-compatibility.md).

ECDSA pins the existing Beacon dependency to `2.1.0-dev.18`. Until coordinated
publication supplies an ethers v6 Beacon package, ECDSA's source checkout uses
its sibling or refreshed bundled deployment scripts and tasks. Packed ECDSA
exports carry the executable bundle too. The explicit
`RANDOM_BEACON_EXPORT_PATH` override allows actual tarball checks without those
fallbacks. The pin is a reproducibility measure, not a claim that the old npm
scripts gained ethers v6 compatibility.

## Remaining gates

| Gate                   | Required work                                                                                                                                                                                                                              |
| ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Release contract       | Agree new major versions/channels and the maintenance period for old consumers. Publish upstream contracts, Beacon, ECDSA, then tbtc-v2. Update ECDSA to the newly published Beacon version before releasing executable ethers v6 exports. |
| Hardhat 3 test runtime | Move to compatible Hardhat 3 ethers/Mocha/Chai plugins after this ethers v6 preparation. Inventory each remaining plugin, task and configuration hook.                                                                                     |
| Shared helpers         | Port or replace the used Hardhat 2 helper APIs. Version 0.7.2 is an ethers v6 preparation path, not a Hardhat 3 implementation. Move temporary patches upstream or explicitly carry them in each consumer.                                 |
| Producer scripts       | Convert the Threshold, Beacon and ECDSA deployment scripts to the agreed ESM/Rocketh API. Convert fixture loading, named-account resolution, tags and dependency ordering together.                                                        |
| Data exports           | Provide a deliberately compatible legacy exporter. Compare actual production-contract output, receipts, start blocks, ABIs, libraries, initialization and artifacts; native Rocketh JSON is a different schema.                            |
| Proxies and upgrades   | Preserve storage validation, manifests, existing-proxy import, shared-admin ownership and upgrade authorization. Account explicitly for the disabled deputy-admin script and manual V2 upgrade path.                                       |
| Consumers              | Run real packed producers in ECDSA and tbtc-v2 with sibling/bundled fallbacks disabled. Run both full suites and fresh integrated deployments. Test the old release line until the last consumer migrates.                                 |

The package versions inspected for the feasibility work were
[hardhat-deploy 2.0.26](https://registry.npmjs.org/hardhat-deploy/2.0.26),
[Rocketh node 0.21.0](https://registry.npmjs.org/@rocketh%2fnode/0.21.0), and
[OpenZeppelin Hardhat Upgrades 4.1.0](https://registry.npmjs.org/@openzeppelin%2fhardhat-upgrades/4.1.0).
The published deploy-v2 CommonJS entry throws a migration error; it is not a
working v1 adapter. Rocketh node supports multiple script directories, giving a
mechanism for external producer discovery. OpenZeppelin supplies a Hardhat 3
plugin, so plugin absence is no longer a blocker; compatibility with this
repository's complete proxy workflow still needs proof.

The isolated feasibility experiment validates a representative packed producer,
legacy exporter and explicit OpenZeppelin v4 proxy/admin flow. That evidence
supports proceeding with staged implementation; it does not establish the full
cross-repository release gate. Coordinate proxy defaults with
[tbtc-v2 #1130](https://github.com/threshold-network/tbtc-v2/issues/1130) and the
client-library direction with
[tbtc-v2 #1128](https://github.com/threshold-network/tbtc-v2/issues/1128).

Keep #4209, #4213 and #4295 open until their respective migrations are complete.

# Hardhat 3 / Rocketh feasibility decision

Status: local feasibility checks passed on 2026-09-08, based on `dev` at
`cbd6e4d2be9ab46d7d6f4096eafc163d36c1a685`. Tracks
[#4295](https://github.com/threshold-network/keep-core/issues/4295).

Proceed with controlled implementation of option B: Rocketh/viem for deployment,
ethers v6 with Mocha/Chai for tests. Keep maintained deploy v1 / Hardhat 2 as the
current release line. Prefer a new versioned ESM/Rocketh release line with a
maintained legacy CJS line until its consumers migrate.

The Fable 5.1 max-effort consultation recommended that release strategy, and the
local compatibility experiment supports the architecture's feasibility. It does
not establish agreement from upstream or downstream maintainers. Same-package
dual publication remains possible if consumer requirements justify its additional
runtime compatibility and release checks.

## Evidence

The reproducible [experiment](../experiments/rocketh-compatibility/README.md)
passed all 10 checks with Node 24.11.1 and npm 11.6.2.

| Boundary | Result |
| --- | --- |
| Published scripts | A producer is packed and installed into node_modules. Its generated source directory is then removed; the consumer loads the packed scripts and their JSON artifact. |
| Cross-package discovery | The Rocketh loader accepts explicit script directories. A consumer file that sorts before its producer still resolves the producer through a tag dependency. An unselected script would throw if executed. |
| Runtime accounts | The producer deploys with the consumer's named accounts, including nondefault deployer and owner indices. |
| Fixtures and reruns | The ethers v6 fixture restores changed contract state. A second Rocketh execution preserves addresses and broadcasts no additional transaction. |
| v1 baseline | A separate Hardhat 2 / deploy-v1 process receives the same compiled artifacts and produces matching addresses and ABIs. |
| Export JSON | A prototype adapter produces byte-identical v1 JSON for both probe contracts, including linkedData. It explicitly restores v1's checksummed address representation. |
| Native Rocketh export | Its top-level chain object and per-contract startBlock differ from the legacy single-network export schema. A compatible exporter is still necessary. |
| Legacy proxies | OpenZeppelin's Hardhat 3 API supports validated implementation deployment, importing two explicit Contracts 4 proxies with a shared admin, rejecting a bad layout, upgrading an existing proxy, transferring admin ownership, and replacing one proxy's admin. |

Both compared JSON files contain 2,279 bytes and have SHA-256
`669bd1ed435bed2b89e2a0677c47a922674ab77b4702b0194722fda6750dbcff`.

The target fixture pins Hardhat 3.15.0, hardhat-deploy 2.0.26, Rocketh and
@rocketh/node 0.21.0, ethers 6.17.0, and @openzeppelin/hardhat-upgrades 4.1.0.
The baseline pins Hardhat 2.26.3 and hardhat-deploy 1.0.4. Full dependency versions
are locked independently.

The v1/ESM comparison deliberately uses the same compiled artifacts. It proves
deployment-engine and export handling for this fixture, not equivalence of
Hardhat 2 and Hardhat 3 compilation of the production contracts.

## Implications for the existing proposal

The external-script question has a tested implementation candidate: configure
the Rocketh loader with resolved directories from installed packages. The actual
upstream packages still need compatible scripts and an agreed public entry point.

A compatible single-network JSON exporter is feasible for the demonstrated
surface. The prototype is not a replacement for `hardhat export-artifacts`,
`--export-all`, or historical production deployment records. File ordering,
optional fields, bytecode, linking, and historical formats require a complete
consumer inventory before this becomes production code.

OpenZeppelin now publishes a Hardhat 3-compatible plugin: 4.1.0 was installed and
exercised here. This removes the question of whether an HH3 plugin exists, but
does not make existing helpers interchangeable. The experiment explicitly creates
legacy proxies/admins; it does not select Contracts 5 defaults or a final helper
adapter. In particular, existing `upgrades.admin.getInstance()` callers require
a deliberate replacement using the appropriate recorded admin address.

## Next implementation steps and release gates

1. Continue the prerequisite stack. #4294 has merged into dev. #4299's ES2020/CJS
   change and #4305's Waffle removal were still open at the time of this experiment.
   Ethers v6 and the real helper/plugin migration remain separate work.
2. Agree the public ESM script entry points and release channels with the upstream
   contracts package and tbtc-v2. Release in dependency order:
   solidity-contracts → random-beacon → ecdsa → tbtc-v2. Pin exact dependencies
   during the transition; ECDSA's current `development` tag must not select an
   incompatible upstream release accidentally.
3. Convert a representative real upstream/beacon path, then a WalletRegistry path,
   exercising actual packed packages. Account for stubs, network tags, linked
   libraries, deployment IDs, skip rules, ownership changes, and restart behavior.
4. Extend the output comparison to the real published artifacts and committed
   mainnet/sepolia records. Preserve existing data contracts; coordinate every
   deliberate difference with consumers. Never regenerate production records as
   a side effect of this experiment.
5. Settle proxy/admin semantics with
   [tbtc-v2 #1130](https://github.com/threshold-network/tbtc-v2/issues/1130).
   Validate actual manifests, existing-proxy upgrades, shared and nondefault admins,
   storage layouts, governance/deputy behavior, and implementation/proxy/admin
   verification. The toy test does not cover these production integrations.
6. Before releasing the full port, run both complete Solidity suites, fresh
   integrated local deployments, and downstream tarball checks. Maintain CJS
   support on the old line until its consumers migrate. Keep #4295 open until
   those acceptance checks pass.

No contract package was migrated or published, and no production deployment or
GitHub mutation was performed by this experiment.

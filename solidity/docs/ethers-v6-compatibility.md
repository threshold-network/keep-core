# Ethers v6 compatibility checks

This prepares the two Solidity packages for
[#4295](https://github.com/threshold-network/keep-core/issues/4295). It retains
Hardhat 2.29.0, hardhat-deploy 1.0.4 and ES2020/CommonJS deployment exports. The
comparison baseline is commit `4f7fa861b`: the strict TypeScript/Waffle-removal
stack plus maintained deploy v1, Node 24/Yarn 4 and ethers 5.8.0. Contract sources,
committed live deployment records and OpenZeppelin manifests are unchanged.

## Runtime and patches

Tested with Node 24.11.1 and Yarn 4.12.0. Both immutable lockfile installs pass.
The active client is ethers 6.17.0, with Hardhat ethers 3.1.3, Chai Matchers 2.1.2,
TypeChain Hardhat 9.1.0, helpers 0.7.2, OpenZeppelin Upgrades 2.5.1 and Tenderly
2.1.1. Mocha/Chai and strict TypeScript are retained. Tenderly automatic
verification is explicitly disabled; existing explicit verification calls remain.

Each package carries four reproducible Yarn patches:

| Package                          | Reason                                                                                                                                                                                      |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| hardhat-helpers 0.7.2            | Accept ethers v6 BaseContract typings, preserve the old proxy deployment receipt fields, and select an explicit v4/v5 ProxyAdmin artifact for preparation with the retained upgrade plugin. |
| OpenZeppelin Upgrades 2.5.1      | Add the chain ID to Etherscan v2 verification queries used by hardhat-verify 2.1.3. This retains the v4 shared-admin creation behavior.                                                     |
| TypeChain ethers-v6 0.5.1        | Handle a contract function named `target`, which conflicts with the ethers v6 base-contract property.                                                                                       |
| Threshold contracts 1.3.0-dev.14 | Port three deployment API uses: the deployed staking address, JSON interface formatting and ZeroAddress. Solidity and artifacts are unchanged by this patch.                                |

The helper's old deploy/OZ peer ranges and OZ 2's verify-v1 peer produce expected
peer warnings. They are not suppressed; the selected patched combinations are
covered by the checks below. Yarn resolutions and devDependency patches do not
propagate automatically into a downstream package installation. Consumers must
adopt the same compatible runtime and upstream fixes before release.

## Test and build evidence

- Both full suites pass: 962 Beacon tests and 673 ECDSA tests, with
  ECDSA's existing 44 pending tests unchanged.
- Four additional Beacon confirmation tests pass using the local ethers provider:
  one confirmation, a mined second confirmation, timeout, and missing transaction.
- Three additional initialization task tests pass: new stake/authorization/operator
  setup, a rerun without transactions, and a stake top-up with increased authorization.
- Both strict TypeScript checks, CommonJS export builds and prepack artifact
  exports pass. Full lint completes with warnings but no errors.
- WalletRegistry upgrade tests exercise layout rejection, existing proxy upgrades,
  shared-admin ownership and authorized versus unauthorized calls.

The confirmation helper uses `getTransaction(...).wait(...)` because the Hardhat
ethers v6 provider does not implement `waitForTransaction`. Published Beacon
scripts include its compiled `export/utils/wait-for-confirmations.js` dependency.
ECDSA's bundled Beacon scripts are regenerated from the same TypeScript sources,
including the missing-approval-function and already-approved guards.

## Production-contract deployment comparison

Fresh deployments use real contracts and external producer scripts on a
non-forked in-process Hardhat network. The clock starts at 2024-01-01 UTC and
advances one second per transaction, so timestamp immutables in SortitionPools
are comparable. Each capture retains full blocks/receipts, deployment records,
exported artifacts and selected readers/code. The comparator checks the EVM
state root after every block, covering storage and balances beyond the selected
owner/governance readers.

| Check                                           | Beacon        | ECDSA                            |
| ----------------------------------------------- | ------------- | -------------------------------- |
| Deployments                                     | 16            | 22                               |
| Transactions with identical resulting EVM state | 28            | 46                               |
| `export.json` bytes                             | Identical     | Identical                        |
| Addresses, runtime code, proxy slots and owners | Identical     | Identical                        |
| Raw deployment-record bytes                     | Identical     | Hash differences described below |
| Artifact inventory                              | Same 51 files | Same 52 files                    |

Export SHA-256 values:

```text
Beacon a7fe677b015af93102a99a276263bb1e8cbc84a06055c6dc98a636b681750083
ECDSA  35e1886a916255779b9a02cda1ee75e84ec1cc6918965347682b8f1f82487333
```

Six ECDSA transactions (blocks 33, 34, 35, 42, 45 and 46) use Hardhat's configured
16,777,216 gas limit through the new ethers signer. The baseline limits were
5,222,326; 443,289; 1,065,225; 15,799,896; 809,383; and 683,960 respectively. Only
the gas-limit field and derived signatures, transaction hashes and block headers
differ. Actual gas used, receipt contents other than hashes, transaction inputs,
fee prices, event data, state roots and ownership all match. The comparator
permits these exact gas changes and translates only their known hash fields;
it rejects other record changes. Raw ECDSA deployment records are consequently
not byte-identical.

In each package, only the TokenStaking artifact changes: OpenZeppelin's newer
compiler integration adds `storageLayout` to this compiler-override artifact.
The layout matches the actual compiler build-info output. Removing that added
field leaves an identical parsed artifact, including ABI, bytecode, deployed
bytecode and metadata. Every other artifact is byte-identical. The comparison
does not silently waive arbitrary artifact differences.

## Reproduce

Install and run `yarn prepack` in both packages at the baseline and candidate.
Use Node 24 and the checked-in lockfiles. From each package directory, capture
into a new directory with the candidate's isolated configuration:

```sh
COMPAT_DIR=/absolute/path/to/candidate/solidity/scripts/ethers-v6-compatibility
HARDHAT_CONFIG="$COMPAT_DIR/hardhat.config.cjs" \
  USE_EXTERNAL_DEPLOY=true MIGRATION_CAPTURE_DIR=/tmp/new-capture \
  node "$COMPAT_DIR/capture.cjs"
```

Leave `TEST_USE_STUBS_BEACON`, `TEST_USE_STUBS_ECDSA`, `FORKING_URL` and
`FORKING_BLOCK` unset. The capture rejects a fork, a public network, test stubs or
an existing destination. Compare matching package captures:

```sh
node "$COMPAT_DIR/compare.cjs" /tmp/v5-capture /tmp/v6-capture --ethers-v6
```

After both candidate prepack steps, run from `solidity/ecdsa`:

```sh
node ../scripts/ethers-v6-compatibility/pack-and-capture.cjs \
  /tmp/new-packed-consumer /tmp/v6-ecdsa-capture
```

This packs and extracts the actual two npm archives and checks their exported
files, artifact counts and Beacon support module. It reuses the installed ECDSA
consumer's plugins; it does not validate dependency installation from scratch.
It executes both producers' compiled deployment scripts through explicit packed
paths and requires byte-identical state, exports, artifacts and deployment
records versus the ethers v6 source capture, without the v5 gas exceptions.
`RANDOM_BEACON_EXPORT_PATH` fails on missing export directories rather than using
sibling or bundled sources. `ECDSA_EXPORT_PATH` is confined to the capture config.

For the committed fallback check, temporarily move Beacon's ignored `export/`
directory aside in **both** trees, capture ECDSA, and restore the directories.
Compare those matching v5/v6 fallback captures with `--ethers-v6`. This check
also passes: all 46 transaction state roots and the selected code/state match,
with the same six gas-limit changes and TokenStaking layout addition described
above. The captured `inputs.json` confirms the bundled deploy path was selected.

The fallback deliberately obtains artifacts from the pinned old Beacon npm
package. Nine Beacon contracts consequently have older compiler metadata than
the sibling build, on both toolchains. Do not compare a fallback capture with a
sibling-build capture and ignore those code differences. The bundled JavaScript
itself must exactly match the newly compiled source. The full ECDSA suite is also
run with the sibling export unavailable.

## Publication lifecycle

Both `prepublishOnly` hooks use helpers 0.7.2's `export-deployment-artifacts`
task. They honor npm's `--network` option and default to `hardhat` when it is
omitted. From a fresh copy of either package with dependencies installed:

```sh
yarn deploy:test --network hardhat --write true
npm publish --dry-run --offline --registry=http://127.0.0.1:1 \
  --access=public --tag=development --network=hardhat
```

This exercises `prepublishOnly`, `prepack`, and ECDSA's `prepare` without
publishing a package. The deployment export requires an empty `artifacts/`
directory. Check that the exported records match `deployments/hardhat/` and
appear in npm's package contents. Use separate fresh copies to check the
omitted-network default and `--network=sepolia` with the checked-in Sepolia
deployment snapshots.

## Limits and release gates

The publishing jobs in
[`npm-random-beacon.yml`](../../.github/workflows/npm-random-beacon.yml) and
[`npm-ecdsa.yml`](../../.github/workflows/npm-ecdsa.yml) are disabled with
`if: ${{ false }}`. This blocks both automatic pushes to `main` and manual
dispatches on any ref, protecting the existing `development` and `latest` npm
tags. Re-enable the jobs in a coordinated release change only after the consumer
runtime migration, upstream fixes, release channel agreement, and ECDSA's pin to
a compatible published Beacon version are ready and the actual packed producers
pass the full ECDSA and tbtc-v2 consumer checks. Local packing and the offline
publication lifecycle checks above remain available while the jobs are disabled.

These checks do not deploy to mainnet/Sepolia, contact explorer verification
services, run the full tbtc-v2 consumer, or authorize package publication. The
manual WalletRegistry V2 script is ported to v6 (and now saves a parsed ABI array)
but its complete operator workflow is not exercised here. The deputy-admin
script remains disabled. The separate Rocketh experiment covers a representative
proxy path, not all of these scripts.

Do not publish these executable exports into an ethers v5 consumer line. Obtain
upstream helper/Threshold fixes or carry the reviewed patches in each consumer,
publish Beacon on the agreed release channel, pin ECDSA to that new release, then
validate the actual tbtc-v2 consumer. See the [migration decision and remaining
gates](hardhat-3-migration.md). Keep #4295 open.

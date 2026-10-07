# Published deployment consumer test

This fixture installs tarballs produced from both packages after `prepack`, then
runs their unmodified `export/deploy` directories in a separate Hardhat project.
It exercises ethers 5 / hardhat-ethers 2 / helpers 0.6 / hardhat-deploy 0.11 and
ethers 6 / hardhat-ethers 3 / helpers 0.7 / hardhat-deploy 1.0.4. The producers
continue to build on ethers 5.

From the repository root, using Node 22 and the producers' installed dependencies:

```sh
(cd solidity/random-beacon && yarn build && yarn prepack)
(cd solidity/ecdsa && yarn build && yarn prepack)
node solidity/deploy-consumer/prepare.js 5
node solidity/deploy-consumer/prepare.js 6
```

`prepare.js` packs both packages, installs them into the consumer, then runs
`smoke.js` and `edge.js` in that order. The tests need the producers' build
output and use the pinned dependency versions in `prepare.js`; transitive
versions are not locked.

An optional second argument chooses the consumer directory, which must be empty.
Otherwise it is created in the system temporary directory. Nothing is published
to npm. Keys, RPC URLs and explorer tokens are removed from the environment of
the consumer's processes.

The consumer compiles the packed WalletRegistry and Allowlist sources for
OpenZeppelin proxy validation. Other contracts use the exported artifacts.
Threshold's real T and TokenStaking contracts are deployed by the fixture;
Threshold's external deploy scripts have a separate compatibility migration.

## smoke.js

`smoke.js` runs on a fresh local chain with distinct deployer, governance,
chaosnetOwner, and ESDM accounts. It enables the Etherscan tag, Chaosnet, the V2
upgrade and Allowlist weights initialization. Contract transactions are real, and
the beacon scripts must ask for 2 confirmations on a verification-tagged network.
It checks ownership (no deployer-held role is left), approvals, authorization,
weights and deployment addresses. It then replays through hardhat-deploy, and
clears the migration journal to cover consumers that only copy deployment JSON.
No account may send a transaction during a replay.

It also covers recovery of a missing or obsolete governance record, and the
approval scripts' read paths: JSON and human-readable ABIs, result shapes,
read failures, and paused or disabled applications. A failed approval
transaction still propagates. Script 16 falls back to the packaged weights for
the network when `ALLOWLIST_WEIGHTS_FILE` is unset, and refuses the override on
mainnet.

Explorer requests are stubbed. The real Etherscan helper logs its errors and
never throws, so the tests inject throwing hooks only to show that a failed
attempt is retried: libraries, the beacon and Tenderly are verified again without
changing deployment addresses or sending transactions. Governance verification
failures are tolerated before ownership transfer, and both hooks run again after
it. Recovered governance records are verified with the network's deploy-time
constructor arguments.

## edge.js

`edge.js` runs on its own fresh chain after `smoke.js`. It covers what the happy
path cannot reach:

- Mainnet skip flags for both approval scripts. A Tenderly failure on mainnet
  stops script 09, and `DISABLE_HARDHAT_VERIFY=true` skips only Etherscan.
- The WalletRegistry V2 upgrade script. Mainnet deploys the implementation and
  writes a Timelock proposal, with the target, calldata and `initializeV2`
  argument decoded, without upgrading. Testnet refuses a signer that does not
  own the ProxyAdmin, treats only a reverting `allowlist()` getter as V1 and
  rethrows other errors, keeps governance and the sortition pool, is a no-op on
  repeat, and reports an Allowlist mismatch.
- Guard clauses against stale or foreign records. Governance moves only once, so
  scripts 10 and 08 stop when the registry or beacon is held by a foreign
  address or the governance contract is linked to another registry or beacon.
  Script 13 stops when the beacon is governed by another contract. Script 04
  stops when the sortition pool is owned by a contract with no beacon record.
  None of these may send a transaction.
- Stale governance and pool records (scripts 09, 07 and 02) are not reused: a
  new contract is deployed for the current registry, beacon or token.

## Known gaps

- The V2 upgrade starts from the current WalletRegistry bytecode with
  `allowlist()` unset. It does not upgrade real V1 bytecode, so storage-layout
  compatibility is not tested here.
- Only the Chaosnet configuration runs.
- The real Etherscan helper swallows verification errors, so a failed Etherscan
  verification on mainnet does not stop a deploy, and no test asserts that it does.
- The ethers 6 lane combines hardhat-deploy 1.0.4 with helpers 0.7, which is
  outside the helpers peer range and needs `--legacy-peer-deps`.

`ALLOWLIST_WEIGHTS_FILE` can select a consumer-owned weights JSON on any network
except mainnet. If unset, script 16 uses the packaged network-specific data
under `export/deploy-data`, which exists for mainnet and sepolia only.

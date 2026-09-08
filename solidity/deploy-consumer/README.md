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

An optional second argument chooses the consumer directory. Otherwise it is
created in the system temporary directory. Nothing is published to npm.

The consumer compiles the packed WalletRegistry and Allowlist sources for
OpenZeppelin proxy validation. Other contracts use the exported artifacts.
Threshold's real T and TokenStaking contracts are deployed by the fixture;
Threshold's external deploy scripts have a separate compatibility migration.

The test runs on a fresh local chain with distinct deployer, governance,
chaosnetOwner, and ESDM accounts. It enables the Etherscan tag, Chaosnet, the V2
upgrade and Allowlist weights initialization. Only explorer verification is
stubbed; deployment confirmation waits and contract transactions are real.
It checks ownership, approvals, authorization, weights and deployment addresses,
then replays through hardhat-deploy and clears the migration journal to
cover consumers that only copy deployment JSON. Account nonces must remain unchanged.
It also tests recovery of a missing governance deployment and legacy approval ABI,
result-shape and read-failure cases. Approval transaction failures still propagate.
Both JSON and human-readable approval ABIs are exercised. Injected library,
beacon and Tenderly verification failures must be retried without changing
deployment addresses or sending transactions.
Governance verification failures are tolerated before ownership transfer; both
verification hooks must run again after transfer, retaining matching deployment
metadata. Missing or obsolete governance records are recovered and verified with
constructor arguments without sending transactions.

`ALLOWLIST_WEIGHTS_FILE` can select a consumer-owned weights JSON. If unset,
script 16 uses the packaged network-specific data under `export/deploy-data`.

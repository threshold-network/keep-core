// Mainnet branches, the WalletRegistry V2 upgrade and the deploy scripts' guard
// clauses. smoke.js covers the happy path and replays; this file covers what a
// fresh local chain cannot reach by itself. It runs after smoke.js, on its own
// chain: its first step clears the deployments that smoke.js left behind.
const assert = require("assert/strict")
const fs = require("fs")
const path = require("path")
const hre = require("hardhat")
const {
  ZERO,
  script,
  equalAddress,
  nonces: allNonces,
  sent,
  run,
} = require("./lib")

// These variables change what the scripts do; a developer shell must not leak in.
for (const name of [
  "UPGRADE_WALLET_REGISTRY_V2",
  "MIGRATE_ALLOWLIST_WEIGHTS",
  "ALLOWLIST_WEIGHTS_FILE",
  "DISABLE_HARDHAT_VERIFY",
]) {
  delete process.env[name]
}

async function main() {
  const { deployments, getNamedAccounts, network, ethers } = hre
  const named = await getNamedAccounts()
  const address = async (name) => (await deployments.get(name)).address
  const read = (name, method, ...args) =>
    deployments.read(name, method, ...args)
  const nonces = () => allNonces(network)
  const asNetwork = (name, overrides = {}) => ({
    ...hre,
    network: { ...network, name },
    ...overrides,
  })
  // Scripts are noisy. Keep their output and show it only if the case fails
  // unexpectedly, or let the case assert on it.
  const captured = async (action, { expectFailure = false } = {}) => {
    const { log, error } = console
    const lines = []
    console.log = console.error = (...args) => lines.push(args.join(" "))
    try {
      return { value: await action(), output: lines.join("\n") }
    } catch (failure) {
      if (!expectFailure) log(lines.join("\n"))
      throw failure
    } finally {
      console.log = log
      console.error = error
    }
  }
  // Replace deployment records for one case and put the originals back.
  const withRecords = async (overrides, action) => {
    const originals = {}
    for (const name of Object.keys(overrides)) {
      originals[name] = await deployments.get(name)
    }
    try {
      for (const [name, record] of Object.entries(overrides)) {
        await deployments.save(name, { ...originals[name], ...record })
      }
      return await action()
    } finally {
      for (const [name, record] of Object.entries(originals)) {
        await deployments.save(name, record)
      }
    }
  }
  const implementation = async () =>
    (
      await hre.upgrades.erc1967.getImplementationAddress(
        await address("WalletRegistry")
      )
    ).toLowerCase()
  // A contract that reports `governance` and nothing else.
  const mockRegistry = async (governance) =>
    (
      await deployments.deploy("RegistryMock", {
        from: named.deployer,
        args: [governance],
      })
    ).address

  hre.helpers.etherscan.verify = async () => {}
  const runTask = hre.run.bind(hre)
  hre.run = (task, args) =>
    task === "verify" ? Promise.resolve() : runTask(task, args)

  await deployments.run("ConsumerDependencies", {
    resetMemory: true,
    deletePreviousDeployments: true,
    writeDeploymentsToFiles: true,
  })
  // Without the opt-in flags the registry stays in its pre-upgrade state.
  await deployments.run(undefined, {
    resetMemory: false,
    deletePreviousDeployments: false,
    writeDeploymentsToFiles: true,
  })
  const registry = await address("WalletRegistry")
  const allowlist = await address("Allowlist")
  const governanceContract = await address("WalletRegistryGovernance")
  const initialImplementation = await implementation()
  equalAddress(await read("WalletRegistry", "allowlist"), ZERO)

  // Production is protected by skip flags, not by the scripts' bodies.
  for (const [name, file] of [
    ["random-beacon", "05_approve_random_beacon_in_token_staking.js"],
    ["ecdsa", "07_approve_wallet_registry.js"],
  ]) {
    const { skip } = script(name, file)
    assert.equal(await skip(asNetwork("mainnet")), true, `${file} on mainnet`)
    assert.equal(await skip(asNetwork("sepolia")), false, `${file} on sepolia`)
  }

  // Mainnet must not execute the upgrade. It deploys the implementation and
  // records the Timelock proposal that governance will schedule.
  const upgrade = script("ecdsa", "17_upgrade_wallet_registry_v2.js")
  const proposalPath = path.join(
    hre.config.paths.root,
    "upgrade-proposal-mainnet.json"
  )
  fs.rmSync(proposalPath, { force: true })
  const beforeMainnet = await nonces()
  assert.equal(
    (await captured(() => upgrade(asNetwork("mainnet")))).value,
    true
  )
  assert.deepEqual(
    sent(beforeMainnet, await nonces()),
    { [named.deployer.toLowerCase()]: 1n },
    "mainnet run should only deploy the implementation"
  )
  assert.equal(await implementation(), initialImplementation)
  equalAddress(await read("WalletRegistry", "allowlist"), ZERO)
  const newImplementation = await address("WalletRegistryV2Implementation")
  const proposal = JSON.parse(fs.readFileSync(proposalPath))
  const proxyAdmin = await hre.upgrades.erc1967.getAdminAddress(registry)
  assert.equal(proposal.network, "mainnet")
  equalAddress(proposal.proxy, registry)
  equalAddress(proposal.proxyAdmin, proxyAdmin)
  equalAddress(proposal.proxyAdminOwner, named.esdm)
  equalAddress(proposal.allowlist, allowlist)
  equalAddress(proposal.newImplementation, newImplementation)
  equalAddress(proposal.calls[0].target, proxyAdmin)
  const adminInterface = (
    await ethers.getContractAt(
      [
        "function upgradeAndCall(address proxy, address implementation, bytes data)",
      ],
      proxyAdmin
    )
  ).interface
  const call = adminInterface.parseTransaction({ data: proposal.calls[0].data })
  assert.equal(call.name, "upgradeAndCall")
  equalAddress(call.args[0], registry)
  equalAddress(call.args[1], newImplementation)
  assert.equal(call.args[2], proposal.verification.initializeV2Data)
  const initializeV2 = (
    await ethers.getContractAt("WalletRegistry", registry)
  ).interface.parseTransaction({ data: call.args[2] })
  assert.equal(initializeV2.name, "initializeV2")
  equalAddress(initializeV2.args[0], allowlist)

  // Testnet refuses to upgrade when the signer is not the ProxyAdmin owner. The
  // stand-in is a real signer, so only the script's own check can refuse it.
  const signers = hre.helpers.signers
  const getNamedSigners = signers.getNamedSigners
  signers.getNamedSigners = async () => ({
    ...(await getNamedSigners.call(signers)),
    esdm: await ethers.getSigner(named.deployer),
  })
  try {
    const before = await nonces()
    const refused = await captured(() => upgrade(hre))
    assert.equal(refused.value, false)
    assert.match(refused.output, /is not the ProxyAdmin owner/)
    assert.deepEqual(sent(before, await nonces()), {}, "non-owner run sent")
    assert.equal(await implementation(), initialImplementation)
  } finally {
    signers.getNamedSigners = getNamedSigners
  }

  // A V1 registry has no allowlist() getter, so the call reverts, and the script
  // must read only a revert as "upgrade needed". The proxy is real; only the
  // read made before the upgrade fails, so the post-upgrade check is genuine.
  const v1Context = (failure) => {
    const calls = { failed: 0 }
    return {
      calls,
      context: asNetwork("hardhat", {
        ethers: {
          ...ethers,
          getContractAt: async (target, ...rest) => {
            const contract = await ethers.getContractAt(target, ...rest)
            if (target !== "WalletRegistry") return contract
            // An empty target: ethers v5 defines contract methods as read-only
            // properties, which a proxy over the contract itself may not replace.
            return new Proxy(
              {},
              {
                get: (_, key) => {
                  if (key !== "allowlist") {
                    const value = Reflect.get(contract, key)
                    return typeof value === "function"
                      ? value.bind(contract)
                      : value
                  }
                  return async () => {
                    if ((await implementation()) !== initialImplementation) {
                      return contract.allowlist()
                    }
                    calls.failed += 1
                    throw failure
                  }
                },
              }
            )
          },
        },
      }),
    }
  }
  const networkError = Object.assign(new Error("connection dropped"), {
    code: "NETWORK_ERROR",
  })
  const dropped = v1Context(networkError)
  await assert.rejects(
    captured(() => upgrade(dropped.context), { expectFailure: true }),
    /connection dropped/
  )
  assert.equal(dropped.calls.failed, 1)
  assert.equal(await implementation(), initialImplementation)

  const revert = Object.assign(new Error("call revert exception"), {
    code: "CALL_EXCEPTION",
  })
  const v1 = v1Context(revert)
  const beforeUpgrade = await nonces()
  assert.equal((await captured(() => upgrade(v1.context))).value, true)
  assert.equal(v1.calls.failed, 1, "the V1 getter failure was not injected")
  assert.deepEqual(
    sent(beforeUpgrade, await nonces()),
    { [named.esdm.toLowerCase()]: 1n },
    "only the ProxyAdmin owner should send the upgrade"
  )
  assert.equal(await implementation(), newImplementation.toLowerCase())
  equalAddress(await read("WalletRegistry", "allowlist"), allowlist)
  // The upgrade must keep the proxy's state and its immutable wiring.
  equalAddress(await read("WalletRegistry", "governance"), governanceContract)
  equalAddress(
    await read("WalletRegistry", "sortitionPool"),
    await address("EcdsaSortitionPool")
  )
  equalAddress(await hre.upgrades.erc1967.getAdminAddress(registry), proxyAdmin)

  // Re-running is a no-op; a different Allowlist is reported, not overwritten.
  const afterUpgrade = await nonces()
  assert.equal((await captured(() => upgrade(hre))).value, true)
  assert.deepEqual(sent(afterUpgrade, await nonces()), {})
  await withRecords({ Allowlist: { address: named.deployer } }, async () => {
    assert.equal((await captured(() => upgrade(hre))).value, false)
  })
  equalAddress(await read("WalletRegistry", "allowlist"), allowlist)

  // Mainnet fails loud when Tenderly fails. DISABLE_HARDHAT_VERIFY skips only
  // the Etherscan hook. (The real Etherscan helper logs its errors and never
  // throws, so its failure cannot reach this wrapper; see the README.)
  const deployGovernance = script(
    "ecdsa",
    "09_deploy_wallet_registry_governance.js"
  )
  const verifyCalls = { etherscan: 0, tenderly: 0 }
  const verifyingHre = (name, tenderlyFails) => ({
    ...hre,
    network: { ...network, name, tags: { ...network.tags, tenderly: true } },
    helpers: {
      ...hre.helpers,
      etherscan: {
        ...hre.helpers.etherscan,
        verify: async () => {
          verifyCalls.etherscan += 1
        },
      },
    },
    tenderly: {
      verify: async () => {
        verifyCalls.tenderly += 1
        if (tenderlyFails) throw new Error("injected mainnet Tenderly failure")
      },
    },
  })
  const governanceNonces = await nonces()
  await assert.rejects(
    deployGovernance(verifyingHre("mainnet", true)),
    /injected mainnet Tenderly failure/
  )
  assert.deepEqual(verifyCalls, { etherscan: 1, tenderly: 1 })
  await deployGovernance(verifyingHre("sepolia", true))
  process.env.DISABLE_HARDHAT_VERIFY = "true"
  try {
    await deployGovernance(verifyingHre("mainnet", false))
  } finally {
    delete process.env.DISABLE_HARDHAT_VERIFY
  }
  assert.deepEqual(
    verifyCalls,
    { etherscan: 2, tenderly: 3 },
    "the flag must skip Etherscan only"
  )
  assert.deepEqual(sent(governanceNonces, await nonces()), {})

  // Guard clauses. Each one protects live contracts from a stale or foreign
  // deployment record, so each must stop before sending a transaction.
  const beaconRecord = await deployments.get("RandomBeacon")
  const guardNonces = await nonces()
  await deployments.delete("RandomBeacon")
  try {
    await assert.rejects(
      script("random-beacon", "04_deploy_random_beacon.js")(hre),
      /BeaconSortitionPool is owned by .*cannot deploy a new RandomBeacon/
    )
  } finally {
    await deployments.save("RandomBeacon", beaconRecord)
  }
  assert.deepEqual(sent(guardNonces, await nonces()), {}, "script 04 sent")

  // Mocks stand in for a registry or beacon that is not the one our governance
  // records point at.
  const unlinkedMock = await mockRegistry(governanceContract)
  const deployerMock = await mockRegistry(named.deployer)
  const foreignMock = await mockRegistry(named.chaosnetOwner)
  const transferRegistryGovernance = script(
    "ecdsa",
    "10_transfer_governance.js"
  )
  const transferBeaconGovernance = script(
    "random-beacon",
    "08_transfer_governance.js"
  )
  const mockNonces = await nonces()
  await withRecords({ WalletRegistry: { address: unlinkedMock } }, async () => {
    await assert.rejects(
      deployGovernance(hre),
      new RegExp(`points to ${registry}, expected ${unlinkedMock}`, "i")
    )
  })
  // Governance moves only once, so a foreign holder or an unlinked governance
  // contract must stop script 10 (and 08 for the beacon) before it transfers.
  await withRecords({ WalletRegistry: { address: foreignMock } }, async () => {
    await assert.rejects(
      transferRegistryGovernance(hre),
      /governance is .*, expected the deployer or/
    )
  })
  await withRecords({ WalletRegistry: { address: deployerMock } }, async () => {
    await assert.rejects(
      transferRegistryGovernance(hre),
      new RegExp(`points to ${registry}, expected ${deployerMock}`, "i")
    )
  })
  await withRecords({ RandomBeacon: { address: foreignMock } }, async () => {
    await assert.rejects(
      transferBeaconGovernance(hre),
      /governance is .*, expected the deployer or/
    )
  })
  await withRecords({ RandomBeacon: { address: deployerMock } }, async () => {
    await assert.rejects(
      transferBeaconGovernance(hre),
      /points to .*, expected/
    )
  })
  // The registry may not be authorized to request through a governance
  // contract that is not the beacon's governance.
  await withRecords(
    {
      WalletRegistry: { address: unlinkedMock },
      RandomBeaconGovernance: { address: named.deployer },
    },
    async () => {
      await assert.rejects(
        script("ecdsa", "13_authorize_in_random_beacon.js")(hre),
        /cannot authorize WalletRegistry as a requester/
      )
    }
  )
  assert.deepEqual(sent(mockNonces, await nonces()), {}, "a guard sent")

  // A governance record left over from an earlier registry, beacon or token
  // must not be reused: a fresh contract is deployed for the current target.
  const governanceRecord = await deployments.get("WalletRegistryGovernance")
  await withRecords({ WalletRegistry: { address: deployerMock } }, async () => {
    await deployGovernance(hre)
    const fresh = await deployments.get("WalletRegistryGovernance")
    assert.notEqual(
      fresh.address.toLowerCase(),
      governanceRecord.address.toLowerCase()
    )
    equalAddress(fresh.args[0], deployerMock)
    await deployments.save("WalletRegistryGovernance", governanceRecord)
  })
  const beaconGovernance = await deployments.get("RandomBeaconGovernance")
  await withRecords({ RandomBeacon: { address: deployerMock } }, async () => {
    await script("random-beacon", "07_deploy_random_beacon_governance.js")(hre)
    const fresh = await deployments.get("RandomBeaconGovernance")
    assert.notEqual(
      fresh.address.toLowerCase(),
      beaconGovernance.address.toLowerCase()
    )
    equalAddress(fresh.args[0], deployerMock)
    await deployments.save("RandomBeaconGovernance", beaconGovernance)
  })
  const poolRecord = await deployments.get("BeaconSortitionPool")
  await withRecords({ T: { address: deployerMock } }, async () => {
    await script("random-beacon", "02_deploy_beacon_sortition_pool.js")(hre)
    const fresh = await deployments.get("BeaconSortitionPool")
    assert.notEqual(
      fresh.address.toLowerCase(),
      poolRecord.address.toLowerCase()
    )
    equalAddress(fresh.args[0], deployerMock)
    await deployments.save("BeaconSortitionPool", poolRecord)
  })
  console.log(
    `PASS: mainnet branches, V2 upgrade and guard clauses, ethers ${
      require("ethers").version
    }`
  )
}
run(main)

const assert = require("assert/strict")
const fs = require("fs")
const path = require("path")
const hre = require("hardhat")

const {
  packageRoot,
  script,
  equalAddress,
  nonces: allNonces,
  run,
} = require("./lib")

async function main() {
  const { deployments, getNamedAccounts, network } = hre
  const named = await getNamedAccounts()
  const address = async (name) => (await deployments.get(name)).address
  const read = (name, method, ...args) =>
    deployments.read(name, method, ...args)
  const nonces = () => allNonces(network)

  // Explorer requests are external, so the verify hook only counts calls. The
  // helper is stubbed because it never throws; throwing stubs below prove that
  // every retry runs the hooks again, not how the real helper fails.
  let verifications = 0
  hre.helpers.etherscan.verify = async (deployment) => {
    assert(deployment.address)
    verifications += 1
  }
  // Record the confirmation depth each script asks for. Beacon contracts on a
  // verification-tagged network must wait for 2 confirmations before verify.
  const beaconNames = [
    "ReimbursementPool",
    "BeaconSortitionPool",
    "BeaconDkgValidator",
    "BLS",
    "BeaconAuthorization",
    "BeaconDkg",
    "BeaconInactivity",
    "RandomBeacon",
    "RandomBeaconGovernance",
    "RandomBeaconChaosnet",
  ]
  const confirmations = {}
  const deploy = deployments.deploy
  deployments.deploy = (name, options) => {
    confirmations[name] = options.waitConfirmations
    return deploy.call(deployments, name, options)
  }
  const runTask = hre.run.bind(hre)
  hre.run = (task, args) =>
    task === "verify" ? Promise.resolve() : runTask(task, args)

  // Exercise the package's shipped data and both opt-in deploy paths, including
  // nonzero weights and the pending Ownable2Step ownership transfer on replay.
  process.env.ALLOWLIST_WEIGHTS_FILE = path.join(
    packageRoot("ecdsa"),
    "export/deploy-data/allowlist-weights-sepolia.json"
  )
  assert(fs.existsSync(process.env.ALLOWLIST_WEIGHTS_FILE))
  process.env.UPGRADE_WALLET_REGISTRY_V2 = "true"
  process.env.MIGRATE_ALLOWLIST_WEIGHTS = "true"

  await deployments.run("ConsumerDependencies", {
    resetMemory: true,
    deletePreviousDeployments: true,
    writeDeploymentsToFiles: true,
  })
  const options = {
    resetMemory: false,
    deletePreviousDeployments: false,
    writeDeploymentsToFiles: true,
  }

  // A failed explorer request happens after deployment and ownership transfer
  // have persisted. Every retry must verify the libraries and beacon without
  // sending another transaction, including a retry after Tenderly fails.
  const beaconVerificationNames = [
    "BLS",
    "BeaconAuthorization",
    "BeaconDkg",
    "BeaconInactivity",
    "RandomBeacon",
  ]
  const verify = hre.helpers.etherscan.verify
  const tenderly = hre.tenderly
  const tenderlyTag = network.tags.tenderly
  let verificationFailure
  let verificationAttempts
  let tenderlyAttempts
  hre.helpers.etherscan.verify = async (deployment) => {
    await verify(deployment)
    for (const name of beaconVerificationNames) {
      const record = await deployments.getOrNull(name)
      if (record && record.address === deployment.address) {
        verificationAttempts.push(name)
        if (verificationFailure === name) {
          throw new Error(`injected verification failure: ${name}`)
        }
      }
    }
  }
  network.tags.tenderly = true
  hre.tenderly = {
    verify: async (deployment) => {
      if (deployment.name === "RandomBeacon") {
        equalAddress(deployment.address, await address("RandomBeacon"))
        tenderlyAttempts += 1
        if (verificationFailure === "Tenderly") {
          throw new Error("injected verification failure: Tenderly")
        }
      }
    },
  }
  try {
    let deployedNonces
    let deployedAddresses
    for (const failure of ["BLS", "RandomBeacon", "Tenderly", undefined]) {
      verificationFailure = failure
      verificationAttempts = []
      tenderlyAttempts = 0
      if (failure) {
        await assert.rejects(
          deployments.run("RandomBeacon", options),
          new RegExp(`injected verification failure: ${failure}`)
        )
      } else {
        await deployments.run("RandomBeacon", options)
      }
      assert.deepEqual(
        verificationAttempts,
        failure === "BLS" ? ["BLS"] : beaconVerificationNames
      )
      assert.equal(
        tenderlyAttempts,
        failure === "BLS" || failure === "RandomBeacon" ? 0 : 1
      )
      equalAddress(
        await read("BeaconSortitionPool", "owner"),
        await address("RandomBeacon")
      )
      const deployed = await Promise.all(beaconVerificationNames.map(address))
      if (deployedNonces) {
        assert.deepEqual(
          await nonces(),
          deployedNonces,
          "verification retry sent transactions"
        )
        assert.deepEqual(deployed, deployedAddresses)
      } else {
        deployedNonces = await nonces()
        deployedAddresses = deployed
      }
    }
  } finally {
    hre.helpers.etherscan.verify = verify
    hre.tenderly = tenderly
    if (tenderlyTag === undefined) delete network.tags.tenderly
    else network.tags.tenderly = tenderlyTag
  }
  // Governance verification failures are tolerated off mainnet, so the loader
  // can transfer governance before an operator retries the failed verification.
  const verifyGovernance = async (action, fail = false) => {
    const attempts = { etherscan: [], tenderly: [], tenderlyAll: [] }
    const previousVerify = hre.helpers.etherscan.verify
    const previousTenderly = hre.tenderly
    const previousTag = network.tags.tenderly
    hre.helpers.etherscan.verify = async (deployment) => {
      await previousVerify(deployment)
      const governance = await deployments.getOrNull("WalletRegistryGovernance")
      if (governance && governance.address === deployment.address) {
        attempts.etherscan.push(deployment)
        if (fail) throw new Error("injected governance Etherscan failure")
      }
    }
    network.tags.tenderly = true
    hre.tenderly = {
      verify: async (deployment) => {
        attempts.tenderlyAll.push(deployment)
        if (deployment.name === "WalletRegistryGovernance") {
          attempts.tenderly.push(deployment)
          if (fail) throw new Error("injected governance Tenderly failure")
        }
      },
    }
    try {
      await action()
    } finally {
      hre.helpers.etherscan.verify = previousVerify
      hre.tenderly = previousTenderly
      if (previousTag === undefined) delete network.tags.tenderly
      else network.tags.tenderly = previousTag
    }
    return attempts
  }
  const assertGovernanceVerification = (attempts, deployment) => {
    // Assert outside the hooks: the testnet helpers intentionally catch failures.
    assert.equal(attempts.etherscan.length, 1)
    equalAddress(attempts.etherscan[0].address, deployment.address)
    assert.deepEqual(attempts.etherscan[0].args, deployment.args)
    assert.deepEqual(attempts.tenderly, [
      { name: "WalletRegistryGovernance", address: deployment.address },
    ])
  }
  const initialGovernanceVerification = await verifyGovernance(
    () => deployments.run(undefined, options),
    true
  )
  const governanceRecord = await deployments.get("WalletRegistryGovernance")
  assertGovernanceVerification(initialGovernanceVerification, governanceRecord)
  // Tenderly must be given the registry proxy, not its implementation.
  const registryProxy = await address("WalletRegistry")
  const tenderlyRegistry = initialGovernanceVerification.tenderlyAll.filter(
    (deployment) => deployment.name === "WalletRegistry"
  )
  assert.equal(tenderlyRegistry.length, 1)
  equalAddress(tenderlyRegistry[0].address, registryProxy)
  equalAddress(
    await read("WalletRegistry", "governance"),
    governanceRecord.address
  )
  const governanceNonces = await nonces()
  const governanceMetadata = JSON.parse(JSON.stringify(governanceRecord))
  const deployGovernance = script(
    "ecdsa",
    "09_deploy_wallet_registry_governance.js"
  )
  assertGovernanceVerification(
    await verifyGovernance(() =>
      deployGovernance({ ...hre, network: { ...network, name: "sepolia" } })
    ),
    governanceRecord
  )
  assert.deepEqual(
    JSON.parse(
      JSON.stringify(await deployments.get("WalletRegistryGovernance"))
    ),
    governanceMetadata,
    "governance verification retry replaced deployment metadata"
  )
  assert.deepEqual(
    await nonces(),
    governanceNonces,
    "governance verification retry sent transactions"
  )

  const beacon = await address("RandomBeacon")
  const registry = await address("WalletRegistry")
  const allowlist = await address("Allowlist")
  equalAddress(await read("BeaconSortitionPool", "owner"), beacon)
  equalAddress(await read("EcdsaSortitionPool", "owner"), registry)
  equalAddress(
    await read("RandomBeacon", "governance"),
    await address("RandomBeaconGovernance")
  )
  equalAddress(
    await read("WalletRegistry", "governance"),
    await address("WalletRegistryGovernance")
  )
  equalAddress(
    await read("WalletRegistryGovernance", "owner"),
    named.governance
  )
  equalAddress(await read("RandomBeaconGovernance", "owner"), named.governance)
  equalAddress(await read("WalletRegistry", "allowlist"), allowlist)
  equalAddress(await read("Allowlist", "pendingOwner"), named.governance)
  equalAddress(await read("ReimbursementPool", "owner"), named.governance)
  // No deployer-held role may survive the run.
  for (const pool of ["BeaconSortitionPool", "EcdsaSortitionPool"]) {
    equalAddress(await read(pool, "chaosnetOwner"), named.chaosnetOwner)
  }
  equalAddress(await read("RandomBeaconChaosnet", "owner"), named.governance)
  const proxyAdmin = await hre.ethers.getContractAt(
    ["function owner() view returns (address)"],
    await hre.upgrades.erc1967.getAdminAddress(registry)
  )
  equalAddress(await proxyAdmin.owner(), named.esdm)
  assert(await read("ReimbursementPool", "isAuthorized", beacon))
  assert(await read("ReimbursementPool", "isAuthorized", registry))
  assert(await read("RandomBeacon", "authorizedRequesters", registry))
  assert(await read("RandomBeaconChaosnet", "authorizedRequesters", registry))
  for (const application of [beacon, registry]) {
    assert.equal(
      (
        await read("TokenStaking", "applicationInfo", application)
      ).status.toString(),
      "1"
    )
  }
  const weights = JSON.parse(
    fs.readFileSync(process.env.ALLOWLIST_WEIGHTS_FILE)
  )
  for (const operator of weights.operators) {
    assert.equal(
      (
        await read(
          "Allowlist",
          "authorizedStake",
          operator.stakingProvider,
          registry
        )
      ).toString(),
      operator.weight
    )
  }
  assert(verifications > 0, "verification-tagged paths were not exercised")
  for (const name of beaconNames) {
    assert(
      confirmations[name] >= 2,
      `${name} waited for ${confirmations[name]}`
    )
  }
  const before = await nonces()
  const beforeAddresses = Object.fromEntries(
    Object.entries(await deployments.all()).map(([name, deployment]) => [
      name,
      deployment.address,
    ])
  )
  await deployments.run(undefined, options)

  // A downstream consumer may copy deployment JSON without the migration journal.
  // Clear only that journal, then use the real loader again so each package's
  // exported artifacts retain their normal resolution priority.
  fs.rmSync(
    path.join(hre.config.paths.deployments, network.name, ".migrations.json"),
    { force: true }
  )
  await deployments.run(undefined, options)
  assert.deepEqual(await nonces(), before, "replay sent transactions")
  assert.deepEqual(
    Object.fromEntries(
      Object.entries(await deployments.all()).map(([name, deployment]) => [
        name,
        deployment.address,
      ])
    ),
    beforeAddresses
  )

  // Without the override, script 16 falls back to the packaged weights named for
  // the network. Mainnet refuses the override and must always use that file.
  const initializeWeights = script(
    "ecdsa",
    "16_initialize_allowlist_weights.js"
  )
  const override = process.env.ALLOWLIST_WEIGHTS_FILE
  await assert.rejects(
    initializeWeights({ ...hre, network: { ...network, name: "mainnet" } }),
    /not allowed on mainnet/
  )
  delete process.env.ALLOWLIST_WEIGHTS_FILE
  try {
    await initializeWeights({
      ...hre,
      network: { ...network, name: "sepolia" },
    })
  } finally {
    process.env.ALLOWLIST_WEIGHTS_FILE = override
  }
  assert.deepEqual(await nonces(), before, "packaged weights sent transactions")

  // Recover and verify governance when the consumer copied only the registry
  // record or also copied an obsolete governance deployment.
  for (const obsolete of [false, true]) {
    if (obsolete) {
      await deployments.save("WalletRegistryGovernance", {
        ...governanceRecord,
        address: named.deployer,
        args: [named.deployer, 1],
      })
    } else {
      await deployments.delete("WalletRegistryGovernance")
    }
    assertGovernanceVerification(
      await verifyGovernance(() => deployGovernance(hre)),
      governanceRecord
    )
    equalAddress(
      await address("WalletRegistryGovernance"),
      beforeAddresses.WalletRegistryGovernance
    )
    assert.deepEqual(
      (await deployments.get("WalletRegistryGovernance")).args,
      governanceRecord.args
    )
    assert.deepEqual(
      await nonces(),
      before,
      "governance recovery deployed an orphan"
    )
  }

  // ABI/status variations cannot be produced by the current TokenStaking
  // artifact. Exercise them at the read/execute boundary of the real exports.
  for (const [name, file] of [
    ["random-beacon", "05_approve_random_beacon_in_token_staking.js"],
    ["ecdsa", "07_approve_wallet_registry.js"],
  ]) {
    const approve = script(name, file)
    let calls = 0
    const reads = []
    const executions = []
    const tokenStaking = await deployments.get("TokenStaking")
    const application = await address(
      name === "ecdsa" ? "WalletRegistry" : "RandomBeacon"
    )
    const context = (
      abi,
      result,
      execute = async (...args) => {
        calls += 1
        executions.push(args)
      }
    ) => ({
      ...hre,
      deployments: {
        ...deployments,
        get: async (contract) =>
          contract === "TokenStaking"
            ? { ...tokenStaking, abi }
            : deployments.get(contract),
        read: async (...args) => {
          reads.push(args)
          if (result instanceof Error) throw result
          return result
        },
        execute,
      },
    })
    const humanReadableAbi = [
      "function approveApplication(address application)",
      "function applicationInfo(address application) view returns (uint8 status)",
    ]
    for (const abi of [tokenStaking.abi, humanReadableAbi]) {
      for (const info of [
        { status: 1 },
        { status: 1n },
        [1],
        [{ toString: () => "1" }],
        // Paused and disabled applications cannot be approved again.
        { status: 2 },
        { status: 3 },
      ]) {
        await approve(context(abi, info))
      }
    }
    await approve(context([], new Error("no getter")))
    await approve(
      context(["event approveApplication(address application)"], null)
    )
    assert.equal(calls, 0)
    await approve(context(tokenStaking.abi, new Error("getter unavailable")))
    await approve(
      context(
        tokenStaking.abi.filter((entry) => entry.name !== "applicationInfo"),
        null
      )
    )
    await approve(context(tokenStaking.abi, { status: 0 }))
    assert.equal(calls, 3)
    await approve(context(humanReadableAbi, { status: 0 }))
    await approve(context(humanReadableAbi, new Error("getter unavailable")))
    await approve(context([humanReadableAbi[0]], null))
    assert.equal(calls, 6)
    await assert.rejects(
      approve(
        context(tokenStaking.abi, new Error("getter unavailable"), async () => {
          throw new Error("Can't approve application")
        })
      ),
      /Can't approve application/
    )
    // The scripts must ask about, and approve, the application they deploy.
    assert(reads.length > 0)
    for (const args of reads) {
      assert.equal(args[0], "TokenStaking")
      assert.equal(args[1], "applicationInfo")
      equalAddress(args[2], application)
    }
    assert.equal(executions.length, 6)
    for (const args of executions) {
      assert.equal(args[0], "TokenStaking")
      assert.equal(args[2], "approveApplication")
      equalAddress(args[3], application)
    }
  }
  console.log(
    `PASS: published exports, ethers ${
      require("ethers").version
    }, fresh deploy + verification retries + replay + governance recovery + approval variants`
  )
}
run(main)

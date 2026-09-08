const assert = require("assert/strict")
const fs = require("fs")
const path = require("path")
const hre = require("hardhat")

const packageRoot = (name) =>
  path.dirname(require.resolve(`@keep-network/${name}/package.json`))
const script = (name, file) =>
  require(path.join(packageRoot(name), "export/deploy", file)).default
const equalAddress = (actual, expected) =>
  assert.equal(actual.toLowerCase(), expected.toLowerCase())

async function main() {
  const { deployments, getNamedAccounts, network } = hre
  const named = await getNamedAccounts()
  const address = async (name) => (await deployments.get(name)).address
  const read = (name, method, ...args) =>
    deployments.read(name, method, ...args)
  const nonces = () =>
    Promise.all(
      Object.values(named).map((account) =>
        network.provider.send("eth_getTransactionCount", [account, "latest"])
      )
    )

  // Verification services are external; retain real deployment waits and check
  // confirmation depth at the verification boundary instead of calling explorers.
  let verifications = 0
  hre.helpers.etherscan.verify = async (deployment) => {
    assert(deployment.address)
    if (deployment.transactionHash) {
      const receipt = await network.provider.send("eth_getTransactionReceipt", [
        deployment.transactionHash,
      ])
      const latest = await network.provider.send("eth_blockNumber")
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
      const beaconDeployments = await Promise.all(
        beaconNames.map((name) => deployments.getOrNull(name))
      )
      const isBeaconDeployment = beaconDeployments.some(
        (record) => record && record.address === deployment.address
      )
      assert(
        Number(latest) - Number(receipt.blockNumber) + 1 >=
          (isBeaconDeployment ? 2 : 1)
      )
    }
    verifications += 1
  }
  const run = hre.run.bind(hre)
  hre.run = (task, args) =>
    task === "verify" ? Promise.resolve() : run(task, args)

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
  await deployments.run(undefined, options)

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

  // Recover governance when the consumer only copied WalletRegistry.json.
  await deployments.delete("WalletRegistryGovernance")
  await script("ecdsa", "09_deploy_wallet_registry_governance.js")(hre)
  equalAddress(
    await address("WalletRegistryGovernance"),
    beforeAddresses.WalletRegistryGovernance
  )
  assert.deepEqual(
    await nonces(),
    before,
    "governance recovery deployed an orphan"
  )

  // ABI/status variations cannot be produced by the current TokenStaking
  // artifact. Exercise them at the read/execute boundary of the real exports.
  for (const [name, file] of [
    ["random-beacon", "05_approve_random_beacon_in_token_staking.js"],
    ["ecdsa", "07_approve_wallet_registry.js"],
  ]) {
    const approve = script(name, file)
    let calls = 0
    const tokenStaking = await deployments.get("TokenStaking")
    const context = (
      abi,
      result,
      execute = async () => {
        calls += 1
      }
    ) => ({
      ...hre,
      deployments: {
        ...deployments,
        get: async (contract) =>
          contract === "TokenStaking"
            ? { ...tokenStaking, abi }
            : deployments.get(contract),
        read: async () => {
          if (result instanceof Error) throw result
          return result
        },
        execute,
      },
    })
    for (const info of [
      { status: 1 },
      { status: 1n },
      [1],
      [{ toString: () => "1" }],
    ]) {
      await approve(context(tokenStaking.abi, info))
    }
    await approve(context([], new Error("no getter")))
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
    await assert.rejects(
      approve(
        context(tokenStaking.abi, new Error("getter unavailable"), async () => {
          throw new Error("Can't approve application")
        })
      ),
      /Can't approve application/
    )
  }
  console.log(
    `PASS: published exports, ethers ${
      require("ethers").version
    }, fresh deploy + replay + governance recovery + approval variants`
  )
}
main().then(
  () => process.exit(0),
  (error) => {
    console.error(
      (error.stack || String(error))
        .split("\n")
        .slice(0, 12)
        .map((line) => line.slice(0, 400))
        .join("\n")
    )
    process.exit(1)
  }
)

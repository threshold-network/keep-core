import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, ethers, helpers, upgrades } = hre
  const { deployer } = await getNamedAccounts()
  const { log } = deployments

  const EcdsaSortitionPool = await deployments.get("EcdsaSortitionPool")
  const TokenStaking = await deployments.get("TokenStaking")
  const ReimbursementPool = await deployments.get("ReimbursementPool")
  
  // Try to get RandomBeacon, or find it from random-beacon package
  let RandomBeacon = await deployments.getOrNull("RandomBeacon")
  if (!RandomBeacon) {
    const fs = require("fs")
    const path = require("path")
    const randomBeaconPath = path.resolve(
      __dirname,
      "../../random-beacon/deployments/development/RandomBeacon.json"
    )
    if (fs.existsSync(randomBeaconPath)) {
      const beaconData = JSON.parse(fs.readFileSync(randomBeaconPath, "utf8"))
      await deployments.save("RandomBeacon", {
        address: beaconData.address,
        abi: beaconData.abi,
      })
      RandomBeacon = await deployments.get("RandomBeacon")
    } else {
      throw new Error("RandomBeacon contract not found")
    }
  }
  
  const EcdsaDkgValidator = await deployments.get("EcdsaDkgValidator")

  const EcdsaInactivity = await deployments.deploy("EcdsaInactivity", {
    from: deployer,
    log: true,
    waitConfirmations: 1,
  })

  // Check if WalletRegistry is already deployed
  const existingWalletRegistry = await deployments.getOrNull("WalletRegistry")
  let walletRegistry, proxyDeployment
  if (existingWalletRegistry) {
    log(`WalletRegistry already deployed at ${existingWalletRegistry.address}, reusing it`)
    walletRegistry = await helpers.contracts.getContract("WalletRegistry")
    // Get proxy deployment info from OpenZeppelin upgrades
    const proxyAdmin = await upgrades.admin.getInstance()
    proxyDeployment = {
      address: walletRegistry.address,
      args: [],
    }
  } else {
    [walletRegistry, proxyDeployment] = await helpers.upgrades.deployProxy(
    "WalletRegistry",
    {
      contractName:
        process.env.TEST_USE_STUBS_ECDSA === "true"
          ? "WalletRegistryStub"
          : undefined,
      initializerArgs: [
        EcdsaDkgValidator.address,
        RandomBeacon.address,
        ReimbursementPool.address,
      ],
      factoryOpts: {
        signer: await ethers.getSigner(deployer),
        libraries: {
          EcdsaInactivity: EcdsaInactivity.address,
        },
      },
      proxyOpts: {
        constructorArgs: [EcdsaSortitionPool.address, TokenStaking.address],
        unsafeAllow: ["external-library-linking"],
        kind: "transparent",
      },
    }
  )
  }

  await helpers.ownable.transferOwnership(
    "EcdsaSortitionPool",
    walletRegistry.address,
    deployer
  )

  if (hre.network.tags.etherscan) {
    await helpers.etherscan.verify(EcdsaInactivity)

    // We use `verify` instead of `verify:verify` as the `verify` task is defined
    // in "@openzeppelin/hardhat-upgrades" to perform Etherscan verification
    // of Proxy and Implementation contracts.
    await hre.run("verify", {
      address: proxyDeployment.address,
      constructorArgsParams: proxyDeployment.args,
    })
  }

  if (hre.network.tags.tenderly) {
    await hre.tenderly.verify({
      name: "WalletRegistry",
      address: walletRegistry.address,
    })
  }
}

export default func

func.tags = ["WalletRegistry"]
func.dependencies = [
  "ReimbursementPool",
  "RandomBeacon",
  "EcdsaSortitionPool",
  "TokenStaking",
  "EcdsaDkgValidator",
]

import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, ethers, helpers } = hre
  const { deployer } = await getNamedAccounts()

  // Skip if WalletRegistry already deployed (for existing testnet deployments)
  const existingWalletRegistry = await deployments.getOrNull("WalletRegistry")
  if (existingWalletRegistry) {
    console.log(`using existing WalletRegistry at ${existingWalletRegistry.address}`)
    return true
  }

  const EcdsaSortitionPool = await deployments.get("EcdsaSortitionPool")
  const TokenStaking = await deployments.get("TokenStaking")
  const ReimbursementPool = await deployments.get("ReimbursementPool")
  const RandomBeacon = await deployments.get("RandomBeacon")
  const EcdsaDkgValidator = await deployments.get("EcdsaDkgValidator")

  const EcdsaInactivity = await deployments.deploy("EcdsaInactivity", {
    from: deployer,
    log: true,
    waitConfirmations: 1,
  })

  const [walletRegistry, proxyDeployment] = await helpers.upgrades.deployProxy(
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
func.id = "deploy_wallet_registry"

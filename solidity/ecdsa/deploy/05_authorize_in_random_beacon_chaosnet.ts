import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments } = hre
  const { deployer } = await getNamedAccounts()
  const { execute } = deployments

  const WalletRegistry = await deployments.get("WalletRegistry")

  if (
    await deployments.read(
      "RandomBeaconChaosnet",
      "authorizedRequesters",
      WalletRegistry.address
    )
  ) {
    return
  }

  await execute(
    "RandomBeaconChaosnet",
    { from: deployer, log: true, waitConfirmations: 1 },
    "setRequesterAuthorization",
    WalletRegistry.address,
    true
  )
}

export default func

func.tags = ["WalletRegistryAuthorizeInBeaconChaosnet"]
func.dependencies = ["UpgradeRandomBeaconChaosnet", "WalletRegistry"]

func.skip = async (hre: HardhatRuntimeEnvironment): Promise<boolean> =>
  !hre.network.tags.useRandomBeaconChaosnet

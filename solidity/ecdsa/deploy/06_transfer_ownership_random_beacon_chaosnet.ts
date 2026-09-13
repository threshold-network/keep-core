import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, helpers } = hre
  const { deployer, governance } = await getNamedAccounts()

  await helpers.ownable.transferOwnership(
    "RandomBeaconChaosnet",
    governance,
    deployer,
  )
}

export default func

func.tags = ["RandomBeaconChaosnetTransferOwnership"]
func.dependencies = ["RandomBeaconChaosnet"]

func.skip = async (hre: HardhatRuntimeEnvironment): Promise<boolean> =>
  !hre.network.tags.useRandomBeaconChaosnet

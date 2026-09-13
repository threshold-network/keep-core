import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments, helpers } = hre
  const { log } = deployments

  const RandomBeaconChaosnet = await deployments.getOrNull(
    "RandomBeaconChaosnet",
  )

  if (
    RandomBeaconChaosnet &&
    helpers.address.isValid(RandomBeaconChaosnet.address)
  ) {
    log(
      `using existing RandomBeaconChaosnet at ${RandomBeaconChaosnet.address}`,
    )
  } else {
    throw new Error("deployed RandomBeaconChaosnet contract not found")
  }
}

export default func

func.tags = ["RandomBeaconChaosnet"]

func.skip = async (hre: HardhatRuntimeEnvironment): Promise<boolean> =>
  !hre.network.tags.useRandomBeaconChaosnet

import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer, governance } = await getNamedAccounts()

  const RandomBeaconGovernance = await deployments.get("RandomBeaconGovernance")

  await helpers.ownable.transferOwnership(
    "RandomBeaconGovernance",
    governance,
    deployer,
  )

  await deployments.execute(
    "RandomBeacon",
    { from: deployer, log: true, waitConfirmations: 1 },
    "transferGovernance",
    RandomBeaconGovernance.address,
  )
}

export default func

func.tags = ["RandomBeaconTransferGovernance"]
func.dependencies = ["RandomBeaconGovernance"]

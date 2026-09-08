import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments } = hre
  const { deployer } = await getNamedAccounts()
  const { execute } = deployments

  const RandomBeacon = await deployments.get("RandomBeacon")

  await execute(
    "ReimbursementPool",
    { from: deployer, log: true, waitConfirmations: 1 },
    "authorize",
    RandomBeacon.address,
  )
}

export default func

func.tags = ["RandomBeaconAuthorize"]
func.dependencies = ["ReimbursementPool", "RandomBeacon"]

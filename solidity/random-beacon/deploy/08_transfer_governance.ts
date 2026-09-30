import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer, governance } = await getNamedAccounts()

  const RandomBeaconGovernance = await deployments.get("RandomBeaconGovernance")

  const owner = await deployments.read("RandomBeaconGovernance", "owner")
  if (helpers.address.equal(owner, deployer)) {
    await helpers.ownable.transferOwnership(
      "RandomBeaconGovernance",
      governance,
      deployer
    )
  }

  const currentGovernance = await deployments.read("RandomBeacon", "governance")
  if (!helpers.address.equal(currentGovernance, deployer)) {
    deployments.log(
      `RandomBeacon governance is already ${currentGovernance}; skipping transfer`
    )
    return
  }

  await deployments.execute(
    "RandomBeacon",
    { from: deployer, log: true, waitConfirmations: 1 },
    "transferGovernance",
    RandomBeaconGovernance.address
  )
}

export default func

func.tags = ["RandomBeaconTransferGovernance"]
func.dependencies = ["RandomBeaconGovernance"]

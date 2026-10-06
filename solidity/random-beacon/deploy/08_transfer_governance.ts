import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer, governance } = await getNamedAccounts()

  const RandomBeaconGovernance = await deployments.get("RandomBeaconGovernance")

  // Governance moves only once, so refuse to hand it to a governance contract
  // that is not ours or is linked to another beacon.
  const currentGovernance = await deployments.read("RandomBeacon", "governance")
  if (
    !helpers.address.equal(currentGovernance, deployer) &&
    !helpers.address.equal(currentGovernance, RandomBeaconGovernance.address)
  ) {
    throw new Error(
      `RandomBeacon governance is ${currentGovernance}, expected the deployer or ${RandomBeaconGovernance.address}`
    )
  }
  const linkedBeacon = await deployments.read(
    "RandomBeaconGovernance",
    "randomBeacon"
  )
  const RandomBeacon = await deployments.get("RandomBeacon")
  if (!helpers.address.equal(linkedBeacon, RandomBeacon.address)) {
    throw new Error(
      `RandomBeaconGovernance at ${RandomBeaconGovernance.address} points to ${linkedBeacon}, expected ${RandomBeacon.address}`
    )
  }

  const owner = await deployments.read("RandomBeaconGovernance", "owner")
  if (
    !helpers.address.equal(owner, deployer) &&
    !helpers.address.equal(owner, governance)
  ) {
    throw new Error(
      `RandomBeaconGovernance is owned by ${owner}, expected the deployer or ${governance}`
    )
  }
  if (helpers.address.equal(owner, deployer)) {
    await helpers.ownable.transferOwnership(
      "RandomBeaconGovernance",
      governance,
      deployer
    )
  }

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

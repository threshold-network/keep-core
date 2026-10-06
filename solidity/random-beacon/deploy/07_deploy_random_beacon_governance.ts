import waitForConfirmations from "../utils/wait-for-confirmations"

import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer } = await getNamedAccounts()

  const RandomBeacon = await deployments.get("RandomBeacon")

  const GOVERNANCE_DELAY = 604_800 // 1 week

  const RandomBeaconGovernance = await deployments.deploy(
    "RandomBeaconGovernance",
    {
      from: deployer,
      args: [RandomBeacon.address, GOVERNANCE_DELAY],
      log: true,
      waitConfirmations: 1,
    },
  )

  if (hre.network.tags.etherscan) {
    if (RandomBeaconGovernance.transactionHash) {
      await waitForConfirmations(
        hre.ethers.provider,
        RandomBeaconGovernance.transactionHash,
        2,
        300000,
      )
    }
    await helpers.etherscan.verify(RandomBeaconGovernance)
  }

  if (hre.network.tags.tenderly) {
    await hre.tenderly.verify({
      name: "RandomBeaconGovernance",
      address: RandomBeaconGovernance.address,
    })
  }
}

export default func

func.tags = ["RandomBeaconGovernance"]
func.dependencies = ["RandomBeacon"]

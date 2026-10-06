import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer } = await getNamedAccounts()

  const RandomBeacon = await deployments.get("RandomBeacon")

  const GOVERNANCE_DELAY = 604_800 // 1 week

  // Reuse a saved record only if it was deployed for this beacon. A record left
  // over from an earlier beacon must not receive its governance.
  const args = [RandomBeacon.address, GOVERNANCE_DELAY]
  const previous = await deployments.getOrNull("RandomBeaconGovernance")
  const sameArgs =
    previous?.args?.length === args.length &&
    previous.args.every(
      (arg, index) =>
        String(arg).toLowerCase() === String(args[index]).toLowerCase()
    )

  const RandomBeaconGovernance = await deployments.deploy(
    "RandomBeaconGovernance",
    {
      from: deployer,
      skipIfAlreadyDeployed: sameArgs,
      args,
      log: true,
      waitConfirmations: hre.network.tags.etherscan ? 2 : 1,
    }
  )

  if (hre.network.tags.etherscan) {
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

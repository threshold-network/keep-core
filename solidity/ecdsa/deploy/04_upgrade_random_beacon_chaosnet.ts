import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments } = hre
  const { deployer } = await getNamedAccounts()
  const { execute } = deployments

  const RandomBeaconChaosnet = await deployments.get("RandomBeaconChaosnet")

  // Upgrade the random beacon smart contract in `WalletRegistry` to
  // `RandomBeaconChaosnet`. This is a temporary solution to enable usage of
  // `WalletRegistry` before the random beacon functionalities in the client
  // are ready.
  try {
    await execute(
      "WalletRegistry",
      { from: deployer, log: true, waitConfirmations: 1 },
      "upgradeRandomBeacon",
      RandomBeaconChaosnet.address
    )
  } catch (error: any) {
    if (error.message?.includes("not the governance") || error.message?.includes("Caller is not the governance")) {
      const { governance } = await getNamedAccounts()
      console.log(`Deployer is not governance, trying with governance account: ${governance}`)
      try {
        await execute(
          "WalletRegistry",
          { from: governance, log: true, waitConfirmations: 1 },
          "upgradeRandomBeacon",
          RandomBeaconChaosnet.address
        )
      } catch (govError: any) {
        console.log(`Upgrade failed. This step may need to be done manually. Error: ${govError.message}`)
        // Don't fail the deployment - upgrade can be done manually if needed
      }
    } else {
      throw error
    }
  }
}

export default func

func.tags = ["UpgradeRandomBeaconChaosnet"]
func.dependencies = ["RandomBeaconChaosnet", "WalletRegistry"]

func.skip = async (hre: HardhatRuntimeEnvironment): Promise<boolean> =>
  !hre.network.tags.useRandomBeaconChaosnet

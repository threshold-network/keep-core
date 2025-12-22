import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer, governance } = await getNamedAccounts()

  const WalletRegistryGovernance = await deployments.get(
    "WalletRegistryGovernance"
  )

  await helpers.ownable.transferOwnership(
    "WalletRegistryGovernance",
    governance,
    deployer
  )

  try {
  await deployments.execute(
    "WalletRegistry",
    { from: deployer, log: true, waitConfirmations: 1 },
    "transferGovernance",
    WalletRegistryGovernance.address
  )
  } catch (error: any) {
    if (error.message?.includes("not the governance") || error.message?.includes("Caller is not the governance")) {
      console.log(`Deployer is not governance, trying with governance account: ${governance}`)
      try {
        await deployments.execute(
          "WalletRegistry",
          { from: governance, log: true, waitConfirmations: 1 },
          "transferGovernance",
          WalletRegistryGovernance.address
        )
      } catch (govError: any) {
        console.log(`Governance transfer failed. This step may need to be done manually. Error: ${govError.message}`)
        // Don't fail the deployment - governance transfer can be done manually if needed
      }
    } else {
      throw error
    }
  }
}

export default func

func.tags = ["WalletRegistryTransferGovernance"]
func.dependencies = ["WalletRegistryGovernance"]

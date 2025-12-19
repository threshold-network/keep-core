import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments } = hre
  const { deployer } = await getNamedAccounts()
  const { execute } = deployments

  const WalletRegistry = await deployments.get("WalletRegistry")

  try {
    await execute(
      "ReimbursementPool",
      { from: deployer, log: true, waitConfirmations: 1 },
      "authorize",
      WalletRegistry.address
    )
  } catch (error: any) {
    // If authorization fails due to ownership, try with governance account
    if (error.message?.includes("not the owner") || error.message?.includes("caller is not the owner")) {
      const { governance } = await getNamedAccounts()
      console.log(`Deployer is not owner, trying with governance account: ${governance}`)
      try {
        await execute(
          "ReimbursementPool",
          { from: governance, log: true, waitConfirmations: 1 },
          "authorize",
          WalletRegistry.address
        )
      } catch (govError: any) {
        console.log(`Authorization failed with governance account. This step may need to be done manually. Error: ${govError.message}`)
        // Don't fail the deployment - authorization can be done manually
      }
    } else {
      throw error
    }
  }
}

export default func

func.tags = ["WalletRegistryAuthorize"]
func.dependencies = ["ReimbursementPool", "WalletRegistry"]

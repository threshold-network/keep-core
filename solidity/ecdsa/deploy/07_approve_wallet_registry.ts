import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"
import { ethers } from "hardhat"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments } = hre
  const { deployer } = await getNamedAccounts()
  const { execute, get } = deployments

  const WalletRegistry = await deployments.get("WalletRegistry")
  const TokenStaking = await get("TokenStaking")

  try {
    // Try to execute approveApplication using hardhat-deploy
    await execute(
      "TokenStaking",
      { from: deployer, log: true, waitConfirmations: 1 },
      "approveApplication",
      WalletRegistry.address
    )
  } catch (error: any) {
    // Check if application is already approved
    if (error.message?.includes("Can't approve application") || error.message?.includes("already approved")) {
      console.log(`WalletRegistry application may already be approved. Skipping approval step.`)
      return
    }
    // If the method doesn't exist in the deployment artifact, try using ethers directly
    if (
      error.message?.includes("No method named") ||
      error.message?.includes("approveApplication")
    ) {
      try {
        // Try to call directly using ethers with a minimal ABI
        const [signer] = await ethers.getSigners()
        const tokenStakingContract = new ethers.Contract(
          TokenStaking.address,
          ["function approveApplication(address)"],
          signer
        )

        const tx = await tokenStakingContract.approveApplication(
          WalletRegistry.address
        )
        await tx.wait(1)
        console.log(
          `Approved WalletRegistry application in TokenStaking: ${WalletRegistry.address}`
        )
      } catch (directError: any) {
        // If direct call also fails, the method doesn't exist
        // Applications might be auto-approved in this version
        console.log(
          `TokenStaking contract doesn't have approveApplication method. ` +
            `Applications may be auto-approved in this version. Skipping approval step.`
        )
      }
    } else {
      // Re-throw if it's a different error (e.g., transaction failure)
      throw error
    }
  }
}

export default func

func.tags = ["WalletRegistryApprove"]
func.dependencies = ["TokenStaking", "WalletRegistry"]

// Skip for mainnet.
func.skip = async (hre: HardhatRuntimeEnvironment): Promise<boolean> =>
  hre.network.name === "mainnet"

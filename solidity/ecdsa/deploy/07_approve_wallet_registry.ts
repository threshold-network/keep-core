import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments } = hre
  const { deployer } = await getNamedAccounts()
  const { execute, get, read, log } = deployments

  const application = await get("WalletRegistry")
  const TokenStaking = await get("TokenStaking")
  const hasFunction = (name: string) =>
    TokenStaking.abi.some(
      (entry) => entry.type === "function" && entry.name === name
    )

  if (!hasFunction("approveApplication")) {
    log(
      "TokenStaking does not have approveApplication; skipping WalletRegistry approval"
    )
    return
  }

  if (hasFunction("applicationInfo")) {
    try {
      const info = await read(
        "TokenStaking",
        "applicationInfo",
        application.address
      )
      // Support named and positional results, including BigNumber and bigint.
      const status = info.status ?? info[0]
      if (status.toString() === "1") {
        log("WalletRegistry already approved in TokenStaking; skipping")
        return
      }
    } catch (error) {
      log(
        `Could not read TokenStaking application status (continuing): ${error}`
      )
    }
  }

  await execute(
    "TokenStaking",
    { from: deployer, log: true, waitConfirmations: 1 },
    "approveApplication",
    application.address
  )
}

export default func

func.tags = ["WalletRegistryApprove"]
func.dependencies = ["TokenStaking", "WalletRegistry"]

// Skip for mainnet (already approved).
func.skip = async (hre: HardhatRuntimeEnvironment): Promise<boolean> =>
  hre.network.name === "mainnet"

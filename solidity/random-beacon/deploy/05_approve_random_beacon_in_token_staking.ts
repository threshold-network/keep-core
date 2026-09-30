import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, ethers } = hre
  const { deployer } = await getNamedAccounts()
  const { execute, get, read, log } = deployments

  const application = await get("RandomBeacon")
  const TokenStaking = await get("TokenStaking")
  // Normalize both JSON and human-readable ABIs using the consumer's ethers.
  const tokenStaking = await ethers.getContractAt(
    TokenStaking.abi,
    TokenStaking.address
  )
  const hasFunction = (name: string) =>
    tokenStaking.interface.fragments.some(
      (fragment) => fragment.type === "function" && fragment.name === name
    )

  if (!hasFunction("approveApplication")) {
    log(
      "TokenStaking does not have approveApplication; skipping RandomBeacon approval"
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
        log("RandomBeacon already approved in TokenStaking; skipping")
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

func.tags = ["RandomBeaconApprove"]
func.dependencies = ["TokenStaking", "RandomBeacon"]

// Skip for mainnet (already approved).
func.skip = async (hre: HardhatRuntimeEnvironment): Promise<boolean> =>
  hre.network.name === "mainnet"

import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

// TokenStaking.ApplicationStatus: NOT_APPROVED=0, APPROVED=1, PAUSED=2, DISABLED=3.
// Only a NOT_APPROVED application can be approved; approving any other status
// reverts, so a replay must skip it.
const APPLICATION_STATUS_NOT_APPROVED = "0"

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
      if (status.toString() !== APPLICATION_STATUS_NOT_APPROVED) {
        log(
          `RandomBeacon already has TokenStaking status ${status}; skipping approval`
        )
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

// Mainnet applications are approved outside this deploy.
func.skip = async (hre: HardhatRuntimeEnvironment): Promise<boolean> =>
  hre.network.name === "mainnet"

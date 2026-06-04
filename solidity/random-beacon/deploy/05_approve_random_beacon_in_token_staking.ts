import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"
import type { utils } from "ethers"

// ApplicationStatus enum: NOT_APPROVED=0, APPROVED=1, PAUSED=2, DISABLED=3
const APPLICATION_STATUS_APPROVED = 1

function ifaceHasFunction(iface: utils.Interface, name: string): boolean {
  try {
    iface.getFunction(name)
    return true
  } catch {
    return false
  }
}

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, ethers } = hre
  const { deployer } = await getNamedAccounts()
  const { execute, get } = deployments

  const RandomBeacon = await deployments.get("RandomBeacon")
  const TokenStaking = await get("TokenStaking")

  const iface = new ethers.utils.Interface(TokenStaking.abi)
  if (!ifaceHasFunction(iface, "approveApplication")) {
    hre.deployments.log(
      "TokenStaking does not have approveApplication (Threshold TokenStaking); skipping"
    )
    return
  }

  // Skip if RandomBeacon is already approved (idempotent for re-runs)
  try {
    const tokenStakingContract = await ethers.getContractAt(
      TokenStaking.abi,
      TokenStaking.address
    )
    if (ifaceHasFunction(iface, "applicationInfo")) {
      const appInfo = await tokenStakingContract.applicationInfo(
        RandomBeacon.address
      )
      if (appInfo.status === APPLICATION_STATUS_APPROVED) {
        hre.deployments.log(
          "RandomBeacon already approved in TokenStaking; skipping"
        )
        return
      }
    }
  } catch (e) {
    hre.deployments.log(
      `Could not read TokenStaking application status (continuing): ${e}`
    )
  }

  await execute(
    "TokenStaking",
    { from: deployer, log: true, waitConfirmations: 1 },
    "approveApplication",
    RandomBeacon.address
  )
}

export default func

func.tags = ["RandomBeaconApprove"]
func.dependencies = ["TokenStaking", "RandomBeacon"]

// Skip for mainnet (already approved).
func.skip = async (hre: HardhatRuntimeEnvironment): Promise<boolean> =>
  hre.network.name === "mainnet"

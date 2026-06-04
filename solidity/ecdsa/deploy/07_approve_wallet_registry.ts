import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"
import type { utils } from "ethers"

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

  const WalletRegistry = await deployments.get("WalletRegistry")
  const TokenStaking = await get("TokenStaking")

  const iface = new ethers.utils.Interface(TokenStaking.abi)
  if (!ifaceHasFunction(iface, "approveApplication")) {
    hre.deployments.log(
      "TokenStaking does not have approveApplication (Threshold TokenStaking); skipping WalletRegistry approval"
    )
    return
  }

  await execute(
    "TokenStaking",
    { from: deployer, log: true, waitConfirmations: 1 },
    "approveApplication",
    WalletRegistry.address
  )
}

export default func

func.tags = ["WalletRegistryApprove"]
func.dependencies = ["TokenStaking", "WalletRegistry"]

// Skip for mainnet.
func.skip = async (hre: HardhatRuntimeEnvironment): Promise<boolean> =>
  hre.network.name === "mainnet"

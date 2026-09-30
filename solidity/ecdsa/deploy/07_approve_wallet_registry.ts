import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"
import type { Interface } from "ethers"

function ifaceHasFunction(iface: Interface, name: string): boolean {
  try {
    return iface.getFunction(name) !== null
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

  const iface = new ethers.Interface(TokenStaking.abi)
  if (!ifaceHasFunction(iface, "approveApplication")) {
    hre.deployments.log(
      "TokenStaking does not have approveApplication (Threshold TokenStaking); skipping WalletRegistry approval",
    )
    return
  }

  await execute(
    "TokenStaking",
    { from: deployer, log: true, waitConfirmations: 1 },
    "approveApplication",
    WalletRegistry.address,
  )
}

export default func

func.tags = ["WalletRegistryApprove"]
func.dependencies = ["TokenStaking", "WalletRegistry"]

// Skip for mainnet.
func.skip = async (hre: HardhatRuntimeEnvironment): Promise<boolean> =>
  hre.network.name === "mainnet"

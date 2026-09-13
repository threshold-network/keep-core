import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer, governance } = await getNamedAccounts()

  const WalletRegistryGovernance = await deployments.get(
    "WalletRegistryGovernance",
  )

  await helpers.ownable.transferOwnership(
    "WalletRegistryGovernance",
    governance,
    deployer,
  )

  await deployments.execute(
    "WalletRegistry",
    { from: deployer, log: true, waitConfirmations: 1 },
    "transferGovernance",
    WalletRegistryGovernance.address,
  )
}

export default func

func.tags = ["WalletRegistryTransferGovernance"]
func.dependencies = ["WalletRegistryGovernance"]

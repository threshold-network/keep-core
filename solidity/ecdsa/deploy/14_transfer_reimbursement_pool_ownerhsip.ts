import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, helpers } = hre
  const { deployer, governance } = await getNamedAccounts()

  await helpers.ownable.transferOwnership(
    "ReimbursementPool",
    governance,
    deployer,
  )
}

export default func

func.tags = ["ReimbursementPoolTransferGovernance"]
func.dependencies = ["ReimbursementPool"]

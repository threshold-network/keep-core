import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer } = await getNamedAccounts()

  const staticGas = 40_800 // gas amount consumed by the refund() + tx cost
  const maxGasPrice = 500_000_000_000 // 500 Gwei

  const ReimbursementPool = await deployments.deploy("ReimbursementPool", {
    from: deployer,
    args: [staticGas, maxGasPrice],
    log: true,
    waitConfirmations: 1,
  })

  if (hre.network.tags.etherscan) {
    if (ReimbursementPool.transactionHash) {
      await hre.ethers.provider.waitForTransaction(
        ReimbursementPool.transactionHash,
        2,
        300000
      )
    }
    await helpers.etherscan.verify(ReimbursementPool)
  }

  if (hre.network.tags.tenderly) {
    await hre.tenderly.verify({
      name: "ReimbursementPool",
      address: ReimbursementPool.address,
    })
  }
}

export default func

func.tags = ["ReimbursementPool"]

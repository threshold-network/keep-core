import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer, chaosnetOwner } = await getNamedAccounts()
  const { execute, log } = deployments
  const { to1e18 } = helpers.number

  const POOL_WEIGHT_DIVISOR = to1e18(1)

  const T = await deployments.get("T")

  const EcdsaSortitionPool = await deployments.deploy("EcdsaSortitionPool", {
    contract: "SortitionPool",
    from: deployer,
    args: [T.address, POOL_WEIGHT_DIVISOR],
    log: true,
    waitConfirmations: 1,
  })

  try {
    await execute(
      "EcdsaSortitionPool",
      { from: deployer, log: true, waitConfirmations: 1 },
      "transferChaosnetOwnerRole",
      chaosnetOwner
    )
  } catch (error: any) {
    if (error.message?.includes("Not the chaosnet owner") || error.message?.includes("not the chaosnet owner")) {
      log("Chaosnet owner role already transferred or deployer is not chaosnet owner. Skipping transfer.")
    } else {
      throw error
    }
  }

  if (hre.network.tags.etherscan) {
    await helpers.etherscan.verify(EcdsaSortitionPool)
  }

  if (hre.network.tags.tenderly) {
    await hre.tenderly.verify({
      name: "EcdsaSortitionPool",
      address: EcdsaSortitionPool.address,
    })
  }
}

export default func

func.tags = ["EcdsaSortitionPool"]
// TokenStaking and T deployments are expected to be resolved from
// @threshold-network/solidity-contracts
func.dependencies = ["TokenStaking", "T"]

import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer, chaosnetOwner } = await getNamedAccounts()
  const { execute } = deployments
  const { to1e18 } = helpers.number

  // Skip if EcdsaSortitionPool already deployed (for existing mainnet/testnet deployments)
  const existingSortitionPool = await deployments.getOrNull("EcdsaSortitionPool")
  if (existingSortitionPool) {
    console.log(`using existing EcdsaSortitionPool at ${existingSortitionPool.address}`)
    return true
  }

  const POOL_WEIGHT_DIVISOR = to1e18(1)

  const T = await deployments.get("T")

  const EcdsaSortitionPool = await deployments.deploy("EcdsaSortitionPool", {
    contract: "SortitionPool",
    from: deployer,
    args: [T.address, POOL_WEIGHT_DIVISOR],
    log: true,
    waitConfirmations: 1,
  })

  await execute(
    "EcdsaSortitionPool",
    { from: deployer, log: true, waitConfirmations: 1 },
    "transferChaosnetOwnerRole",
    chaosnetOwner
  )

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
func.id = "deploy_ecdsa_sortition_pool"

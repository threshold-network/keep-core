import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer, chaosnetOwner } = await getNamedAccounts()
  const { execute } = deployments

  const POOL_WEIGHT_DIVISOR = "1000000000000000000"

  const T = await deployments.get("T")

  // Reuse a saved record only if it was deployed for this token, so a
  // redeployed T never ends up with a pool bound to the old one.
  const args = [T.address, POOL_WEIGHT_DIVISOR]
  const previous = await deployments.getOrNull("BeaconSortitionPool")
  const sameArgs =
    previous?.args?.length === args.length &&
    previous.args.every(
      (arg, index) =>
        String(arg).toLowerCase() === String(args[index]).toLowerCase()
    )

  const BeaconSortitionPool = await deployments.deploy("BeaconSortitionPool", {
    contract: "SortitionPool",
    skipIfAlreadyDeployed: sameArgs,
    from: deployer,
    args,
    log: true,
    waitConfirmations: hre.network.tags.etherscan ? 2 : 1,
  })

  const currentChaosnetOwner = await deployments.read(
    "BeaconSortitionPool",
    "chaosnetOwner"
  )
  if (!helpers.address.equal(currentChaosnetOwner, chaosnetOwner)) {
    await execute(
      "BeaconSortitionPool",
      { from: deployer, log: true, waitConfirmations: 1 },
      "transferChaosnetOwnerRole",
      chaosnetOwner
    )
  }

  if (hre.network.tags.etherscan) {
    await helpers.etherscan.verify(BeaconSortitionPool)
  }

  if (hre.network.tags.tenderly) {
    await hre.tenderly.verify({
      name: "BeaconSortitionPool",
      address: BeaconSortitionPool.address,
    })
  }
}

export default func

func.tags = ["BeaconSortitionPool"]
// TokenStaking and T deployments are expected to be resolved from
// @threshold-network/solidity-contracts
func.dependencies = ["TokenStaking", "T"]

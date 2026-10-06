import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer, governance } = await getNamedAccounts()

  const WalletRegistryGovernance = await deployments.get(
    "WalletRegistryGovernance"
  )

  // Governance moves only once, so refuse to hand it to a governance contract
  // that is not ours or is linked to another registry.
  const currentGovernance = await deployments.read(
    "WalletRegistry",
    "governance"
  )
  if (
    !helpers.address.equal(currentGovernance, deployer) &&
    !helpers.address.equal(currentGovernance, WalletRegistryGovernance.address)
  ) {
    throw new Error(
      `WalletRegistry governance is ${currentGovernance}, expected the deployer or ${WalletRegistryGovernance.address}`
    )
  }
  const linkedRegistry = await deployments.read(
    "WalletRegistryGovernance",
    "walletRegistry"
  )
  const WalletRegistry = await deployments.get("WalletRegistry")
  if (!helpers.address.equal(linkedRegistry, WalletRegistry.address)) {
    throw new Error(
      `WalletRegistryGovernance at ${WalletRegistryGovernance.address} points to ${linkedRegistry}, expected ${WalletRegistry.address}`
    )
  }

  const owner = await deployments.read("WalletRegistryGovernance", "owner")
  if (
    !helpers.address.equal(owner, deployer) &&
    !helpers.address.equal(owner, governance)
  ) {
    throw new Error(
      `WalletRegistryGovernance is owned by ${owner}, expected the deployer or ${governance}`
    )
  }
  if (helpers.address.equal(owner, deployer)) {
    await helpers.ownable.transferOwnership(
      "WalletRegistryGovernance",
      governance,
      deployer
    )
  }

  if (!helpers.address.equal(currentGovernance, deployer)) {
    deployments.log(
      `WalletRegistry governance is already ${currentGovernance}; skipping transfer`
    )
    return
  }

  await deployments.execute(
    "WalletRegistry",
    { from: deployer, log: true, waitConfirmations: 1 },
    "transferGovernance",
    WalletRegistryGovernance.address
  )
}

export default func

func.tags = ["WalletRegistryTransferGovernance"]
func.dependencies = ["WalletRegistryGovernance"]

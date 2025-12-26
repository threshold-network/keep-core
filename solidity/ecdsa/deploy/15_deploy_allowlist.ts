import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, ethers, helpers } = hre

  const { deployer, governance } = await getNamedAccounts()

  // Get the WalletRegistry deployment - this will be replaced by Allowlist
  // but we still need it for initialization
  const WalletRegistry = await deployments.get("WalletRegistry")

  // Deploy the Allowlist contract using upgradeable proxy pattern
  const [allowlist, proxyDeployment] = await helpers.upgrades.deployProxy(
    "Allowlist",
    {
      initializerArgs: [WalletRegistry.address],
      factoryOpts: {
        signer: await ethers.getSigner(deployer),
      },
      proxyOpts: {
        kind: "transparent",
      },
    }
  )

  // IMPORTANT: Do NOT transfer ownership here!
  // Allowlist uses Ownable2StepUpgradeable which requires two steps:
  // 1. transferOwnership(newOwner) - sets pendingOwner
  // 2. acceptOwnership() - new owner must call to complete transfer
  //
  // If we transfer here, script 16 would fail because:
  // - governance becomes pendingOwner (not owner)
  // - deployer is still the actual owner
  // - script 16 would try to use governance signer → onlyOwner fails
  //
  // Ownership transfer is handled at the END of script 16 after weights are set.

  // Log deployment information
  console.log(`Allowlist deployed at: ${allowlist.address}`)
  console.log(
    `Allowlist proxy admin: ${proxyDeployment.receipt.contractAddress}`
  )
  console.log(`Allowlist owner: ${await allowlist.owner()} (deployer)`)
  if (governance && governance !== deployer) {
    console.log(`Ownership will be transferred to governance (${governance}) after weights initialization`)
  }

  return true
}

export default func

func.tags = ["Allowlist"]
func.dependencies = ["WalletRegistry"]
func.id = "deploy_allowlist"

// Skip if Allowlist is already deployed
func.skip = async (hre: HardhatRuntimeEnvironment) => {
  const { deployments } = hre
  const existingAllowlist = await deployments.getOrNull("Allowlist")
  if (existingAllowlist) {
    console.log(`Skipping Allowlist deployment - already deployed at ${existingAllowlist.address}`)
    return true
  }
  return false
}

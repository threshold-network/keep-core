import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

/**
 * Upgrades WalletRegistry to V2 with Allowlist integration.
 *
 * This script performs an atomic upgrade of the WalletRegistry proxy:
 * 1. Deploys new WalletRegistry implementation
 * 2. Upgrades proxy to new implementation
 * 3. Atomically calls initializeV2(allowlist) during upgrade
 *
 * IMPORTANT: This must be executed by the proxy admin owner (esdm).
 * For mainnet, this will be executed via governance proposal.
 *
 * The atomic upgradeToAndCall pattern prevents front-running attacks
 * by ensuring the upgrade and initialization happen in a single transaction.
 */
const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { deployments, ethers, helpers } = hre

  // Get named signers - esdm is the proxy admin owner
  const { esdm, deployer } = await helpers.signers.getNamedSigners()

  // Get required contract deployments
  const EcdsaSortitionPool = await deployments.get("EcdsaSortitionPool")
  const TokenStaking = await deployments.get("TokenStaking")
  const Allowlist = await deployments.get("Allowlist")
  const EcdsaInactivity = await deployments.get("EcdsaInactivity")

  console.log("=== WALLET REGISTRY V2 UPGRADE ===")
  console.log()
  console.log("Contract addresses:")
  console.log(`  EcdsaSortitionPool: ${EcdsaSortitionPool.address}`)
  console.log(`  TokenStaking: ${TokenStaking.address}`)
  console.log(`  Allowlist: ${Allowlist.address}`)
  console.log(`  EcdsaInactivity (library): ${EcdsaInactivity.address}`)
  console.log()
  console.log(`Proxy admin owner (esdm): ${esdm.address}`)
  console.log()

  // Check current WalletRegistry state before upgrade
  const walletRegistryDeployment = await deployments.get("WalletRegistry")
  const walletRegistryBefore = await ethers.getContractAt(
    "WalletRegistry",
    walletRegistryDeployment.address
  )

  console.log("Current WalletRegistry state:")
  console.log(`  Address: ${walletRegistryDeployment.address}`)

  // Check if already upgraded (allowlist is set)
  const currentAllowlist = await walletRegistryBefore.allowlist()
  if (currentAllowlist !== ethers.constants.AddressZero) {
    console.log(`  Allowlist: ${currentAllowlist}`)
    console.log()
    console.log("WalletRegistry is already upgraded to V2!")

    if (currentAllowlist.toLowerCase() === Allowlist.address.toLowerCase()) {
      console.log("Allowlist address matches. No upgrade needed.")
      return true
    } else {
      console.error("ERROR: Allowlist address mismatch!")
      console.error(`  Current: ${currentAllowlist}`)
      console.error(`  Expected: ${Allowlist.address}`)
      return false
    }
  }

  console.log(`  Allowlist: ${currentAllowlist} (not set - upgrade needed)`)
  console.log()

  // Verify governance state is preserved
  const governanceBefore = await walletRegistryBefore.governance()
  console.log(`  Governance: ${governanceBefore}`)
  console.log()

  console.log("Performing atomic upgrade with initializeV2...")
  console.log()

  try {
    // Upgrade WalletRegistry to V2 with atomic initializeV2 call
    // The upgrade uses the same contract name since we're upgrading in-place
    // (the V2 functionality is already in WalletRegistry.sol via initializeV2)
    const walletRegistryV2 = await helpers.upgrades.upgradeProxy(
      "WalletRegistry",
      "WalletRegistry",
      {
        factoryOpts: {
          signer: esdm,
          libraries: {
            EcdsaInactivity: EcdsaInactivity.address,
          },
        },
        proxyOpts: {
          // Constructor args for immutable variables (same as current deployment)
          constructorArgs: [EcdsaSortitionPool.address, TokenStaking.address],
          // Atomic call to initializeV2 during upgrade
          call: {
            fn: "initializeV2",
            args: [Allowlist.address],
          },
          unsafeAllow: ["external-library-linking"],
        },
      }
    )

    console.log("Upgrade successful!")
    console.log()

    // Verify upgrade
    console.log("=== POST-UPGRADE VERIFICATION ===")
    console.log()

    const newAllowlist = await walletRegistryV2.allowlist()
    const governanceAfter = await walletRegistryV2.governance()
    const sortitionPool = await walletRegistryV2.sortitionPool()
    const minimumAuthorization = await walletRegistryV2.minimumAuthorization()

    console.log("WalletRegistry V2 state:")
    console.log(`  Address: ${walletRegistryV2.address}`)
    console.log(`  Allowlist: ${newAllowlist}`)
    console.log(`  Governance: ${governanceAfter}`)
    console.log(`  SortitionPool: ${sortitionPool}`)
    console.log(`  MinimumAuthorization: ${ethers.utils.formatEther(minimumAuthorization)} T`)
    console.log()

    // Validation checks
    let validationPassed = true

    if (newAllowlist.toLowerCase() !== Allowlist.address.toLowerCase()) {
      console.error("VALIDATION FAILED: Allowlist address mismatch!")
      validationPassed = false
    } else {
      console.log("Allowlist address: VERIFIED")
    }

    if (governanceAfter.toLowerCase() !== governanceBefore.toLowerCase()) {
      console.error("VALIDATION FAILED: Governance address changed!")
      validationPassed = false
    } else {
      console.log("Governance preserved: VERIFIED")
    }

    if (walletRegistryV2.address.toLowerCase() !== walletRegistryDeployment.address.toLowerCase()) {
      console.error("VALIDATION FAILED: Proxy address changed!")
      validationPassed = false
    } else {
      console.log("Proxy address preserved: VERIFIED")
    }

    console.log()

    if (validationPassed) {
      console.log("UPGRADE COMPLETED SUCCESSFULLY!")
      console.log()
      console.log("Next steps:")
      console.log("1. Run script 16 to initialize Allowlist with operator weights")
      console.log("   MIGRATE_ALLOWLIST_WEIGHTS=true npx hardhat deploy --tags InitializeAllowlistWeights")
      console.log("2. Operators can now call joinSortitionPool() to join with Allowlist weights")
      return true
    } else {
      console.error("UPGRADE COMPLETED WITH VALIDATION ERRORS!")
      return false
    }
  } catch (error: any) {
    console.error("UPGRADE FAILED!")
    console.error()
    console.error("Error:", error.message)
    console.error()

    if (error.message.includes("Ownable: caller is not the owner")) {
      console.error("The signer is not the proxy admin owner.")
      console.error("Ensure the esdm account is correctly configured and has ownership.")
    } else if (error.message.includes("already initialized")) {
      console.error("The contract has already been initialized.")
      console.error("This upgrade may have already been performed.")
    }

    return false
  }
}

export default func

func.tags = ["UpgradeWalletRegistryV2"]
func.dependencies = ["Allowlist", "WalletRegistry", "EcdsaInactivity"]

// Only run this script when explicitly requested
func.skip = async () => !process.env.UPGRADE_WALLET_REGISTRY_V2

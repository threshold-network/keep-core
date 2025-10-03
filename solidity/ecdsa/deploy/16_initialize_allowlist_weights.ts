import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, ethers } = hre
  const { deployer, governance } = await getNamedAccounts()

  // Get contract instances
  const allowlistDeployment = await deployments.get("Allowlist")
  const walletRegistryDeployment = await deployments.get("WalletRegistry")
  const tokenStakingDeployment = await deployments.get("TokenStaking")

  const allowlist = await ethers.getContractAt(
    "Allowlist",
    allowlistDeployment.address
  )
  const walletRegistry = await ethers.getContractAt(
    "WalletRegistry",
    walletRegistryDeployment.address
  )
  const tokenStaking = await ethers.getContractAt(
    "TokenStaking",
    tokenStakingDeployment.address
  )

  // Get the governance signer (owner of Allowlist)
  const governanceSigner = await ethers.getSigner(governance || deployer)

  console.log("Starting beta staker migration to Allowlist...")

  // Query all existing beta stakers from WalletRegistry
  // We'll look for AuthorizationIncreased events to find all staking providers
  const authorizationFilter = walletRegistry.filters.AuthorizationIncreased()
  const authorizationEvents = await walletRegistry.queryFilter(
    authorizationFilter
  )

  // Extract unique staking providers
  const stakingProviders = new Set<string>()
  for (const event of authorizationEvents) {
    if (event.args && event.args.stakingProvider) {
      stakingProviders.add(event.args.stakingProvider)
    }
  }

  console.log(`Found ${stakingProviders.size} unique staking providers`)

  // For each staking provider, get their current authorized stake and add to Allowlist
  const migrationResults = []

  for (const stakingProvider of Array.from(stakingProviders)) {
    try {
      // Get current authorized stake from TokenStaking
      const authorizedStake = await tokenStaking.authorizedStake(
        stakingProvider,
        walletRegistry.address
      )

      if (authorizedStake.gt(0)) {
        console.log(
          `Migrating staking provider ${stakingProvider} with weight ${ethers.utils.formatEther(
            authorizedStake
          )} T`
        )

        // Add to Allowlist with current weight
        const tx = await allowlist
          .connect(governanceSigner)
          .addStakingProvider(stakingProvider, authorizedStake)
        await tx.wait()

        migrationResults.push({
          stakingProvider,
          weight: authorizedStake,
          status: "success",
        })

        console.log(`✓ Successfully migrated ${stakingProvider}`)
      } else {
        console.log(`Skipping ${stakingProvider} - no authorized stake`)
        migrationResults.push({
          stakingProvider,
          weight: authorizedStake,
          status: "skipped - no stake",
        })
      }
    } catch (error) {
      console.error(`✗ Failed to migrate ${stakingProvider}:`, error.message)
      migrationResults.push({
        stakingProvider,
        weight: "0",
        status: `failed: ${error.message}`,
      })
    }
  }

  // Summary
  const successful = migrationResults.filter(
    (r) => r.status === "success"
  ).length
  const failed = migrationResults.filter((r) =>
    r.status.startsWith("failed")
  ).length
  const skipped = migrationResults.filter((r) =>
    r.status.startsWith("skipped")
  ).length

  console.log("\n=== Migration Summary ===")
  console.log(`Total staking providers found: ${stakingProviders.size}`)
  console.log(`Successfully migrated: ${successful}`)
  console.log(`Failed: ${failed}`)
  console.log(`Skipped: ${skipped}`)

  // Save migration results to file for record keeping
  const fs = require("fs")
  const path = require("path")
  const resultsPath = path.join(__dirname, "../migration-results.json")
  fs.writeFileSync(
    resultsPath,
    JSON.stringify(
      {
        timestamp: new Date().toISOString(),
        network: hre.network.name,
        results: migrationResults,
      },
      null,
      2
    )
  )

  console.log(`Migration results saved to: ${resultsPath}`)

  if (failed > 0) {
    console.warn(
      `⚠️  Migration completed with ${failed} failures. Please review the results.`
    )
  } else {
    console.log("✅ Migration completed successfully!")
  }

  return true
}

export default func

func.tags = ["InitializeAllowlistWeights"]
func.dependencies = ["Allowlist"]

// Only run this script when explicitly requested
func.skip = async (hre) => !process.env.MIGRATE_ALLOWLIST_WEIGHTS

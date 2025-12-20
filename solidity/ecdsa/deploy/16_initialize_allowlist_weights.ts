import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"
import * as fs from "fs"
import * as path from "path"

// Type definitions for the weights JSON structure
interface OperatorWeight {
  identification: string
  stakingProvider: string
  operator: string
  operatorType: string
  providerGroup: string | null
  originalTStake: number
  accumulatedTStake: number
  weight: string
  poolWeightAfterDivision: number
  weightNote: string
}

interface WeightsData {
  metadata: {
    generatedAt: string
    source: string
    note: string
  }
  summary: {
    operatorsAddedToAllowlist: number
    operatorsNotAdded: number
    totalOriginalTStake: number
    totalAccumulatedTStake: number
    stakeIncreaseFromConsolidation: number
    totalWeight: string
  }
  operators: OperatorWeight[]
  betaStakerConsolidation: Array<{
    providerGroup: string
    stayingOperator: string
    accumulatedStake: number
    consolidatedOperators: number
  }>
}

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, ethers } = hre
  const { deployer, governance } = await getNamedAccounts()

  // Load pre-calculated weights from JSON
  // These weights include accumulated stakes from consolidated beta stakers
  const weightsPath = path.join(__dirname, "data/allowlist-weights.json")

  if (!fs.existsSync(weightsPath)) {
    throw new Error(
      `Weights file not found at ${weightsPath}. ` +
      `Please ensure allowlist-weights.json exists in deploy/data/`
    )
  }

  const weightsData: WeightsData = JSON.parse(
    fs.readFileSync(weightsPath, "utf8")
  )

  console.log("=== ALLOWLIST INITIALIZATION ===")
  console.log(`Source: ${weightsData.metadata.source}`)
  console.log(`Generated: ${weightsData.metadata.generatedAt}`)
  console.log(`Note: ${weightsData.metadata.note}`)
  console.log()

  // Get contract instances
  const allowlistDeployment = await deployments.get("Allowlist")
  const walletRegistryDeployment = await deployments.get("WalletRegistry")

  const allowlist = await ethers.getContractAt(
    "Allowlist",
    allowlistDeployment.address
  )
  const walletRegistry = await ethers.getContractAt(
    "WalletRegistry",
    walletRegistryDeployment.address
  )

  // Get the actual owner of Allowlist (should be deployer at this point)
  // Allowlist uses Ownable2StepUpgradeable, so ownership transfer is two-step.
  // Script 15 does NOT transfer ownership, keeping deployer as owner.
  // Ownership transfer to governance happens at the END of this script.
  const currentOwner = await allowlist.owner()
  const ownerSigner = await ethers.getSigner(currentOwner)

  console.log(`Allowlist address: ${allowlist.address}`)
  console.log(`WalletRegistry address: ${walletRegistry.address}`)
  console.log(`Allowlist owner: ${currentOwner}`)
  console.log(`Owner signer: ${await ownerSigner.getAddress()}`)
  console.log()

  // Display beta staker consolidation summary
  console.log("=== BETA STAKER CONSOLIDATION ===")
  for (const consolidation of weightsData.betaStakerConsolidation) {
    console.log(
      `${consolidation.providerGroup}: ` +
      `${consolidation.consolidatedOperators} operators -> 1 ` +
      `(accumulated: ${consolidation.accumulatedStake.toLocaleString()} T)`
    )
  }
  console.log()

  // Add each operator to the Allowlist with their accumulated weight
  console.log("=== ADDING OPERATORS TO ALLOWLIST ===")
  console.log(`Total operators to add: ${weightsData.operators.length}`)
  console.log()

  const migrationResults: Array<{
    stakingProvider: string
    identification: string
    weight: string
    accumulatedTStake: number
    status: string
    txHash?: string
  }> = []

  for (const op of weightsData.operators) {
    try {
      // Check if already added
      const existingWeight = await allowlist.authorizedStake(
        op.stakingProvider,
        ethers.constants.AddressZero
      )

      if (existingWeight.gt(0)) {
        console.log(
          `Skipping ${op.identification} (${op.stakingProvider.slice(0, 10)}...) - already in Allowlist`
        )
        migrationResults.push({
          stakingProvider: op.stakingProvider,
          identification: op.identification,
          weight: op.weight,
          accumulatedTStake: op.accumulatedTStake,
          status: "skipped - already exists",
        })
        continue
      }

      console.log(
        `Adding ${op.identification} (${op.operatorType}):`
      )
      console.log(`  Staking Provider: ${op.stakingProvider}`)
      console.log(`  Weight: ${op.accumulatedTStake.toLocaleString()} T`)
      if (op.providerGroup) {
        console.log(`  Note: ${op.weightNote}`)
      }

      // Add to Allowlist with accumulated weight
      const tx = await allowlist
        .connect(ownerSigner)
        .addStakingProvider(op.stakingProvider, op.weight)

      const receipt = await tx.wait()

      console.log(`  TX: ${tx.hash}`)
      console.log(`  Status: SUCCESS`)
      console.log()

      migrationResults.push({
        stakingProvider: op.stakingProvider,
        identification: op.identification,
        weight: op.weight,
        accumulatedTStake: op.accumulatedTStake,
        status: "success",
        txHash: tx.hash,
      })
    } catch (error: any) {
      console.error(`  FAILED: ${error.message}`)
      console.log()

      migrationResults.push({
        stakingProvider: op.stakingProvider,
        identification: op.identification,
        weight: op.weight,
        accumulatedTStake: op.accumulatedTStake,
        status: `failed: ${error.message}`,
      })
    }
  }

  // Verify WalletRegistry V2 is initialized with Allowlist
  console.log("=== VERIFYING WALLET REGISTRY V2 ===")

  const currentAllowlist = await walletRegistry.allowlist()

  if (currentAllowlist === ethers.constants.AddressZero) {
    console.error("ERROR: WalletRegistry V2 is not initialized!")
    console.error()
    console.error("Please run the upgrade script first:")
    console.error("  UPGRADE_WALLET_REGISTRY_V2=true npx hardhat deploy --tags UpgradeWalletRegistryV2")
    console.error()
    console.error("The upgrade script atomically upgrades WalletRegistry and calls initializeV2.")
    return false
  }

  if (currentAllowlist.toLowerCase() !== allowlist.address.toLowerCase()) {
    console.error("ERROR: WalletRegistry is initialized with a different Allowlist!")
    console.error(`  Current: ${currentAllowlist}`)
    console.error(`  Expected: ${allowlist.address}`)
    return false
  }

  console.log(`WalletRegistry V2 initialized with Allowlist: ${currentAllowlist}`)
  console.log("Verification: PASSED")

  // Summary
  console.log()
  console.log("=== MIGRATION SUMMARY ===")

  const successful = migrationResults.filter((r) => r.status === "success").length
  const failed = migrationResults.filter((r) => r.status.startsWith("failed")).length
  const skipped = migrationResults.filter((r) => r.status.startsWith("skipped")).length

  console.log(`Total operators processed: ${migrationResults.length}`)
  console.log(`Successfully added: ${successful}`)
  console.log(`Skipped (already exists): ${skipped}`)
  console.log(`Failed: ${failed}`)
  console.log()
  console.log(`Total accumulated stake: ${weightsData.summary.totalAccumulatedTStake.toLocaleString()} T`)
  console.log(`Stake increase from consolidation: +${weightsData.summary.stakeIncreaseFromConsolidation.toLocaleString()} T`)

  // Save migration results
  const resultsPath = path.join(__dirname, "../migration-results.json")
  fs.writeFileSync(
    resultsPath,
    JSON.stringify(
      {
        timestamp: new Date().toISOString(),
        network: hre.network.name,
        allowlistAddress: allowlist.address,
        walletRegistryAddress: walletRegistry.address,
        weightsSource: weightsData.metadata.source,
        weightsGeneratedAt: weightsData.metadata.generatedAt,
        summary: {
          totalOperators: migrationResults.length,
          successful,
          skipped,
          failed,
          totalAccumulatedStake: weightsData.summary.totalAccumulatedTStake,
          stakeIncreaseFromConsolidation: weightsData.summary.stakeIncreaseFromConsolidation,
        },
        betaStakerConsolidation: weightsData.betaStakerConsolidation,
        results: migrationResults,
      },
      null,
      2
    )
  )

  console.log()
  console.log(`Migration results saved to: ${resultsPath}`)

  if (failed > 0) {
    console.warn()
    console.warn(`WARNING: Migration completed with ${failed} failures.`)
    console.warn("Please review the results and retry failed operations.")
    return false
  }

  // Transfer ownership to governance (Ownable2StepUpgradeable - two-step process)
  // Step 1: Current owner calls transferOwnership() to set pendingOwner
  // Step 2: Governance must call acceptOwnership() to complete the transfer
  if (governance && governance.toLowerCase() !== currentOwner.toLowerCase()) {
    console.log()
    console.log("=== INITIATING OWNERSHIP TRANSFER ===")
    console.log()
    console.log("Allowlist uses Ownable2StepUpgradeable (two-step transfer):")
    console.log("  Step 1: transferOwnership(governance) - sets pendingOwner")
    console.log("  Step 2: governance calls acceptOwnership() - completes transfer")
    console.log()

    try {
      console.log(`Initiating transfer from ${currentOwner} to ${governance}...`)

      const tx = await allowlist
        .connect(ownerSigner)
        .transferOwnership(governance)

      await tx.wait()

      console.log(`  TX: ${tx.hash}`)
      console.log(`  Status: SUCCESS`)
      console.log()

      const pendingOwner = await allowlist.pendingOwner()
      console.log(`Pending owner set to: ${pendingOwner}`)
      console.log()
      console.log("IMPORTANT: Governance must call Allowlist.acceptOwnership() to complete the transfer!")
      console.log("Until then, the current owner remains: " + await allowlist.owner())
    } catch (error: any) {
      console.error(`  FAILED: ${error.message}`)
      console.log()
      console.warn("WARNING: Ownership transfer failed. Manual intervention required.")
    }
  } else {
    console.log()
    console.log("Ownership transfer not needed (owner is already governance or same account).")
  }

  console.log()
  console.log("Migration completed successfully!")

  return true
}

export default func

func.tags = ["InitializeAllowlistWeights"]
func.dependencies = ["Allowlist", "UpgradeWalletRegistryV2"]

// Only run this script when explicitly requested
func.skip = async () => !process.env.MIGRATE_ALLOWLIST_WEIGHTS

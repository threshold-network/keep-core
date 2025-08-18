import { ethers } from "hardhat"
import { Command } from "commander"

const program = new Command()

// Authoritative consolidation plan: Keep 1 operator per entity, consolidate the rest
const OPERATORS_TO_CONSOLIDATE = [
  // Staked (consolidate 5 of 6)
  "0xcc957f683a7e3093388946d03193eee10086b900",
  "0xeae5790c6ee3b6425f39d3fd33644a7cb90c75a5", 
  "0x02faa4286ef91247f8d09f36618d4694717f76bb",
  "0xba1ac67539c09adde63335635869c86f8e463514",
  "0xa6e3a08fae33898fc31c4f6c7a584827d809352d",
  
  // P2P (consolidate 5 of 6)
  "0xda08c16c86b78cd56cb10fdc0370efc549d8638b",
  "0xc0b851dcbf00ba59d8b1f490af93dec4275cffcc",
  "0x372626ff774573e82eb7d4545ee96f68f75aaff6", 
  "0xb88a62417eb9e6320af7620be0cfbe2dddd435a5",
  "0xb78f9efe4f713feefcab466d2ee41972a0e45205",
  
  // Boar (consolidate 5 of 6)  
  "0x6dee1fd2b29e2214a4f9ab9ba5f3d17c8cb56d11",
  "0x5838636dcdd92113998fecbcdedf5b0d8beb4920",
  "0xa7baca5a92842689359fb1782e75d6eff59152e6",
  "0xe4a3492c8b085ab5edb6fdae329f172056f6b04e", 
  "0xca5ac1b59796be580820e9c66d395977d4f7c3c0",
  
  // NUCO (consolidate 1 of 2)
  "0xB6C7382f67c6866597ff3D8902220f1505Bd6825"
]

const OPERATORS_TO_KEEP = [
  "0x16fcc54e027a342f0683263eb43cd9af1bd72169", // Staked #1
  "0xdc09db6e5da859edeb7fc7bdcf47545056dc35f7", // P2P #1
  "0xaafc71044c2b832dddfcedb0ae99695b0367dc57", // Boar #1
  "0xD7F138ccF194ca2F49c28870b3b5F556B57Fb8b7"  // NUCO #1
]

const ENTITY_MAPPINGS: Record<string, string> = {
  // Staked
  "0x16fcc54e027a342f0683263eb43cd9af1bd72169": "Staked",
  "0xcc957f683a7e3093388946d03193eee10086b900": "Staked", 
  "0xeae5790c6ee3b6425f39d3fd33644a7cb90c75a5": "Staked",
  "0x02faa4286ef91247f8d09f36618d4694717f76bb": "Staked",
  "0xba1ac67539c09adde63335635869c86f8e463514": "Staked",
  "0xa6e3a08fae33898fc31c4f6c7a584827d809352d": "Staked",
  
  // P2P
  "0xdc09db6e5da859edeb7fc7bdcf47545056dc35f7": "P2P",
  "0xda08c16c86b78cd56cb10fdc0370efc549d8638b": "P2P",
  "0xc0b851dcbf00ba59d8b1f490af93dec4275cffcc": "P2P", 
  "0x372626ff774573e82eb7d4545ee96f68f75aaff6": "P2P",
  "0xb88a62417eb9e6320af7620be0cfbe2dddd435a5": "P2P",
  "0xb78f9efe4f713feefcab466d2ee41972a0e45205": "P2P",
  
  // Boar
  "0xaafc71044c2b832dddfcedb0ae99695b0367dc57": "Boar",
  "0x6dee1fd2b29e2214a4f9ab9ba5f3d17c8cb56d11": "Boar",
  "0x5838636dcdd92113998fecbcdedf5b0d8beb4920": "Boar",
  "0xa7baca5a92842689359fb1782e75d6eff59152e6": "Boar", 
  "0xe4a3492c8b085ab5edb6fdae329f172056f6b04e": "Boar",
  "0xca5ac1b59796be580820e9c66d395977d4f7c3c0": "Boar",
  
  // NUCO
  "0xD7F138ccF194ca2F49c28870b3b5F556B57Fb8b7": "NUCO",
  "0xB6C7382f67c6866597ff3D8902220f1505Bd6825": "NUCO"
}

program
  .name("consolidate-beta-stakers")
  .description("One-click beta staker consolidation: 20 → 4 operators")
  .version("1.0.0")

program
  .command("execute")
  .description("Execute the full consolidation: check, consolidate, verify")
  .requiredOption("-a, --allowlist <address>", "Allowlist contract address")
  .option("-s, --signer <name>", "Named signer to use", "governance")
  .option("--dry-run", "Show what would be done without executing")
  .action(async (options) => {
    console.log("🚀 BETA STAKER CONSOLIDATION")
    console.log("=".repeat(50))
    console.log(`Allowlist: ${options.allowlist}`)
    console.log(`Signer: ${options.signer}`)
    console.log(`Dry run: ${options.dryRun ? "YES" : "NO"}`)
    console.log()

    const allowlist = await ethers.getContractAt("Allowlist", options.allowlist)
    const signer = await ethers.getNamedSigner(options.signer)

    // Step 1: Check current state
    console.log("📋 STEP 1: Checking current operator state...")
    console.log("-".repeat(30))

    const allOperators = [...OPERATORS_TO_KEEP, ...OPERATORS_TO_CONSOLIDATE]
    const currentStates: Record<string, { weight: string; entity: string }> = {}

    for (const operator of allOperators) {
      try {
        const weight = await allowlist.authorizedStake(
          operator,
          ethers.constants.AddressZero
        )
        const entity = ENTITY_MAPPINGS[operator.toLowerCase()] || "UNKNOWN"
        
        currentStates[operator] = {
          weight: ethers.utils.formatEther(weight),
          entity
        }
        
        const status = OPERATORS_TO_CONSOLIDATE.includes(operator) ? "→ CONSOLIDATE" : "✅ KEEP"
        console.log(`${operator} (${entity}): ${ethers.utils.formatEther(weight)} T ${status}`)
      } catch (error: any) {
        console.error(`❌ Error checking ${operator}: ${error.message}`)
        process.exit(1)
      }
    }

    // Validation
    const missingOperators = allOperators.filter(op => 
      !currentStates[op] || currentStates[op].weight === "0.0"
    )
    
    if (missingOperators.length > 0) {
      console.log(`\n❌ Missing operators in Allowlist:`)
      missingOperators.forEach(op => console.log(`  ${op}`))
      process.exit(1)
    }

    console.log(`\n✅ Found all ${allOperators.length} expected operators`)

    // Step 2: Execute consolidation
    console.log("\n⚙️  STEP 2: Executing consolidation...")
    console.log("-".repeat(30))

    const results: any[] = []
    let successCount = 0
    let skipCount = 0
    let failCount = 0

    for (const operator of OPERATORS_TO_CONSOLIDATE) {
      const entity = currentStates[operator].entity
      const currentWeight = currentStates[operator].weight
      
      console.log(`\nConsolidating ${operator} (${entity})...`)
      console.log(`  Current weight: ${currentWeight} T`)

      if (currentWeight === "0.0") {
        console.log(`  ✓ Already consolidated`)
        results.push({ operator, entity, status: "already_zero" })
        skipCount++
        continue
      }

      if (options.dryRun) {
        console.log(`  [DRY RUN] Would set weight to 0`)
        results.push({ operator, entity, status: "dry_run", currentWeight })
        successCount++
        continue
      }

      try {
        const tx = await allowlist
          .connect(signer)
          .requestWeightDecrease(operator, 0)
        
        console.log(`  Transaction: ${tx.hash}`)
        await tx.wait()
        
        console.log(`  ✅ Weight decrease requested`)
        results.push({ 
          operator, 
          entity, 
          status: "success", 
          transaction: tx.hash,
          previousWeight: currentWeight 
        })
        successCount++
        
      } catch (error: any) {
        console.error(`  ❌ Failed: ${error.message}`)
        results.push({ operator, entity, status: "failed", error: error.message })
        failCount++
      }
    }

    // Step 3: Verify results
    console.log("\n🔍 STEP 3: Verification...")
    console.log("-".repeat(30))

    if (!options.dryRun && successCount > 0) {
      console.log("Checking pending weight decreases...")
      
      for (const operator of OPERATORS_TO_CONSOLIDATE) {
        try {
          const providerInfo = await allowlist.stakingProviders(operator)
          const pendingWeight = providerInfo.pendingNewWeight
          
          if (pendingWeight.eq(0)) {
            console.log(`✅ ${operator}: Pending decrease to 0`)
          } else {
            console.log(`⚠️  ${operator}: No pending decrease found`)
          }
        } catch (error) {
          console.log(`❌ ${operator}: Error checking pending decrease`)
        }
      }
    }

    // Summary
    console.log("\n📊 CONSOLIDATION SUMMARY")
    console.log("=".repeat(50))
    console.log(`Total operators processed: ${OPERATORS_TO_CONSOLIDATE.length}`)
    console.log(`Successful: ${successCount}`)
    console.log(`Skipped: ${skipCount}`)
    console.log(`Failed: ${failCount}`)
    console.log()
    console.log("Operators remaining active:")
    OPERATORS_TO_KEEP.forEach(op => {
      const entity = currentStates[op]?.entity || "UNKNOWN"
      console.log(`  ✅ ${op} (${entity})`)
    })

    if (!options.dryRun && successCount > 0) {
      console.log("\n⚠️  NEXT STEPS:")
      console.log("- Weight decreases have been requested")
      console.log("- WalletRegistry will approve these after the decrease delay")
      console.log("- Operators will be unable to create new wallets once approved")
      console.log("- Existing wallets will continue operating and drain naturally")
      console.log("- Monitor progress monthly until operators reach zero custody")
    }

    // Save results
    const fs = require("fs")
    const resultsFile = `consolidation-results-${Date.now()}.json`
    fs.writeFileSync(
      resultsFile,
      JSON.stringify({
        timestamp: new Date().toISOString(),
        dryRun: options.dryRun,
        summary: { total: OPERATORS_TO_CONSOLIDATE.length, successful: successCount, skipped: skipCount, failed: failCount },
        operatorsToKeep: OPERATORS_TO_KEEP,
        operatorsConsolidated: OPERATORS_TO_CONSOLIDATE,
        results
      }, null, 2)
    )

    console.log(`\n💾 Results saved to: ${resultsFile}`)
    
    if (failCount === 0) {
      console.log("\n🎉 CONSOLIDATION COMPLETE!")
      console.log("Beta staker reduction: 20 → 4 operators (80% reduction)")
    }
  })

program
  .command("status")
  .description("Check current status of all operators")
  .requiredOption("-a, --allowlist <address>", "Allowlist contract address")
  .action(async (options) => {
    const allowlist = await ethers.getContractAt("Allowlist", options.allowlist)
    
    console.log("CURRENT OPERATOR STATUS")
    console.log("=".repeat(50))
    
    const allOperators = [...OPERATORS_TO_KEEP, ...OPERATORS_TO_CONSOLIDATE]
    
    for (const operator of allOperators) {
      const entity = ENTITY_MAPPINGS[operator.toLowerCase()] || "UNKNOWN"
      const weight = await allowlist.authorizedStake(operator, ethers.constants.AddressZero)
      const providerInfo = await allowlist.stakingProviders(operator)
      const pendingWeight = providerInfo.pendingNewWeight
      
      const status = OPERATORS_TO_CONSOLIDATE.includes(operator) ? "CONSOLIDATE" : "KEEP"
      const pending = pendingWeight.gt(0) ? ` (pending: ${ethers.utils.formatEther(pendingWeight)})` : ""
      
      console.log(`${operator} (${entity})`)
      console.log(`  Weight: ${ethers.utils.formatEther(weight)} T${pending}`)
      console.log(`  Plan: ${status}`)
    }
  })

program.parse()

export { OPERATORS_TO_CONSOLIDATE, OPERATORS_TO_KEEP }
import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

/**
 * Upgrades WalletRegistry to V2 with Allowlist integration.
 *
 * For TESTNET (Sepolia):
 * - Deploys new implementation
 * - Executes upgrade directly via ProxyAdmin
 *
 * For MAINNET:
 * - Deploys new implementation
 * - Outputs calldata for governance to schedule via Timelock
 * - Does NOT execute the upgrade (must go through 24h Timelock)
 *
 * The atomic upgradeToAndCall pattern prevents front-running attacks
 * by ensuring the upgrade and initialization happen in a single transaction.
 */
const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { deployments, ethers, helpers, network } = hre

  const isMainnet = network.name === "mainnet"

  // Get named signers - esdm is the proxy admin owner
  const { esdm, deployer } = await helpers.signers.getNamedSigners()

  // Get required contract deployments
  const EcdsaSortitionPool = await deployments.get("EcdsaSortitionPool")
  const TokenStaking = await deployments.get("TokenStaking")
  const Allowlist = await deployments.get("Allowlist")
  const EcdsaInactivity = await deployments.get("EcdsaInactivity")

  console.log("=== WALLET REGISTRY V2 UPGRADE ===")
  console.log()
  console.log(`Network: ${network.name} (${isMainnet ? "MAINNET" : "TESTNET"})`)
  console.log()
  console.log("Contract addresses:")
  console.log(`  EcdsaSortitionPool: ${EcdsaSortitionPool.address}`)
  console.log(`  TokenStaking: ${TokenStaking.address}`)
  console.log(`  Allowlist: ${Allowlist.address}`)
  console.log(`  EcdsaInactivity (library): ${EcdsaInactivity.address}`)
  console.log()

  // Check current WalletRegistry state before upgrade
  const walletRegistryDeployment = await deployments.get("WalletRegistry")
  const walletRegistryBefore = await ethers.getContractAt(
    "WalletRegistry",
    walletRegistryDeployment.address
  )

  console.log("Current WalletRegistry state:")
  console.log(`  Proxy: ${walletRegistryDeployment.address}`)

  // Check if already upgraded (allowlist is set)
  // Note: V1 doesn't have allowlist() function, so we need to handle that case
  let currentAllowlist = ethers.constants.AddressZero
  try {
    currentAllowlist = await walletRegistryBefore.allowlist()
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
  } catch (error) {
    // V1 doesn't have allowlist() function - upgrade is needed
    console.log("  Allowlist: not found (V1 detected - upgrade needed)")
  }

  // Get ProxyAdmin address from EIP-1967 slot
  const ADMIN_SLOT = "0xb53127684a568b3173ae13b9f8a6016e243e63b6e8ee1178d6a717850b5d6103"
  const proxyAdminSlot = await ethers.provider.getStorageAt(
    walletRegistryDeployment.address,
    ADMIN_SLOT
  )
  const proxyAdminAddress = ethers.utils.getAddress("0x" + proxyAdminSlot.slice(-40))
  console.log(`  ProxyAdmin: ${proxyAdminAddress}`)

  // Get current implementation
  const IMPL_SLOT = "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc"
  const implSlot = await ethers.provider.getStorageAt(
    walletRegistryDeployment.address,
    IMPL_SLOT
  )
  const currentImpl = ethers.utils.getAddress("0x" + implSlot.slice(-40))
  console.log(`  Current Implementation: ${currentImpl}`)

  // Verify governance state is preserved
  const governanceBefore = await walletRegistryBefore.governance()
  console.log(`  Governance: ${governanceBefore}`)
  console.log()

  // Deploy new implementation
  console.log("=== DEPLOYING NEW IMPLEMENTATION ===")
  console.log()

  const WalletRegistryFactory = await ethers.getContractFactory("WalletRegistry", {
    signer: deployer,
    libraries: {
      EcdsaInactivity: EcdsaInactivity.address,
    },
  })

  // Deploy implementation with constructor args (immutable variables)
  const newImplementation = await WalletRegistryFactory.deploy(
    EcdsaSortitionPool.address,
    TokenStaking.address
  )
  await newImplementation.deployed()

  console.log(`New implementation deployed: ${newImplementation.address}`)
  console.log(`  TX: ${newImplementation.deployTransaction.hash}`)
  console.log()

  // Save deployment artifact
  await deployments.save("WalletRegistryV2Implementation", {
    address: newImplementation.address,
    abi: WalletRegistryFactory.interface.format("json") as any,
    transactionHash: newImplementation.deployTransaction.hash,
  })

  // Encode initializeV2 call
  const initializeV2Data = WalletRegistryFactory.interface.encodeFunctionData(
    "initializeV2",
    [Allowlist.address]
  )

  // Encode upgradeAndCall for ProxyAdmin
  const proxyAdminInterface = new ethers.utils.Interface([
    "function upgradeAndCall(address proxy, address implementation, bytes calldata data) external payable",
    "function owner() external view returns (address)"
  ])

  const upgradeCalldata = proxyAdminInterface.encodeFunctionData(
    "upgradeAndCall",
    [walletRegistryDeployment.address, newImplementation.address, initializeV2Data]
  )

  // Get ProxyAdmin owner
  const proxyAdmin = await ethers.getContractAt(
    ["function owner() view returns (address)"],
    proxyAdminAddress
  )
  const proxyAdminOwner = await proxyAdmin.owner()
  console.log(`ProxyAdmin owner: ${proxyAdminOwner}`)

  if (isMainnet) {
    // MAINNET: Output governance proposal data
    console.log()
    console.log("=== MAINNET GOVERNANCE PROPOSAL ===")
    console.log()
    console.log("The upgrade must be executed through the Timelock (24h delay).")
    console.log()
    console.log("Step 1: Schedule the upgrade via Timelock")
    console.log("----------------------------------------")
    console.log()
    console.log("Target (ProxyAdmin):", proxyAdminAddress)
    console.log("Value: 0")
    console.log("Data (upgradeAndCall):")
    console.log(upgradeCalldata)
    console.log()
    console.log("Or using cast:")
    console.log("```")
    console.log(`cast send ${proxyAdminAddress} \\`)
    console.log(`  "upgradeAndCall(address,address,bytes)" \\`)
    console.log(`  ${walletRegistryDeployment.address} \\`)
    console.log(`  ${newImplementation.address} \\`)
    console.log(`  ${initializeV2Data} \\`)
    console.log(`  --rpc-url $CHAIN_API_URL \\`)
    console.log(`  --private-key <TIMELOCK_EXECUTOR_KEY>`)
    console.log("```")
    console.log()
    console.log("NOTE: This call must come from the Timelock after scheduling!")
    console.log()
    console.log("Step 2: Wait 24 hours")
    console.log("---------------------")
    console.log()
    console.log("Step 3: Execute the upgrade")
    console.log("---------------------------")
    console.log("Execute the scheduled transaction via Timelock.")
    console.log()

    // Save proposal data for reference
    const proposalData = {
      network: network.name,
      timestamp: new Date().toISOString(),
      description: "Upgrade WalletRegistry to V2 with Allowlist integration",
      newImplementation: newImplementation.address,
      proxy: walletRegistryDeployment.address,
      proxyAdmin: proxyAdminAddress,
      proxyAdminOwner: proxyAdminOwner,
      allowlist: Allowlist.address,
      timelockDelay: "24 hours",
      calls: [
        {
          target: proxyAdminAddress,
          value: "0",
          data: upgradeCalldata,
          description: "ProxyAdmin.upgradeAndCall(proxy, newImpl, initializeV2Data)"
        }
      ],
      verification: {
        initializeV2Args: [Allowlist.address],
        initializeV2Data: initializeV2Data,
      }
    }

    const fs = require("fs")
    const path = require("path")
    const proposalPath = path.join(__dirname, "../upgrade-proposal-mainnet.json")
    fs.writeFileSync(proposalPath, JSON.stringify(proposalData, null, 2))
    console.log(`Proposal data saved to: ${proposalPath}`)

    return true
  } else {
    // TESTNET: Execute upgrade directly
    console.log("=== EXECUTING UPGRADE (TESTNET) ===")
    console.log()

    try {
      // Check if esdm is the ProxyAdmin owner
      if (proxyAdminOwner.toLowerCase() !== esdm.address.toLowerCase()) {
        console.error(`ERROR: esdm (${esdm.address}) is not the ProxyAdmin owner`)
        console.error(`ProxyAdmin owner is: ${proxyAdminOwner}`)
        console.log()
        console.log("Manual upgrade command:")
        console.log("```")
        console.log(`cast send ${proxyAdminAddress} \\`)
        console.log(`  "upgradeAndCall(address,address,bytes)" \\`)
        console.log(`  ${walletRegistryDeployment.address} \\`)
        console.log(`  ${newImplementation.address} \\`)
        console.log(`  "${initializeV2Data}" \\`)
        console.log(`  --rpc-url $CHAIN_API_URL \\`)
        console.log(`  --private-key <OWNER_KEY>`)
        console.log("```")
        return false
      }

      // Execute upgrade via ProxyAdmin
      const proxyAdminContract = await ethers.getContractAt(
        ["function upgradeAndCall(address proxy, address implementation, bytes calldata data) external payable"],
        proxyAdminAddress,
        esdm
      )

      console.log("Calling ProxyAdmin.upgradeAndCall()...")
      const tx = await proxyAdminContract.upgradeAndCall(
        walletRegistryDeployment.address,
        newImplementation.address,
        initializeV2Data
      )

      console.log(`TX: ${tx.hash}`)
      await tx.wait()
      console.log("Upgrade executed successfully!")
      console.log()

      // Verify upgrade
      console.log("=== POST-UPGRADE VERIFICATION ===")
      console.log()

      const walletRegistryV2 = await ethers.getContractAt(
        "WalletRegistry",
        walletRegistryDeployment.address
      )

      const newAllowlist = await walletRegistryV2.allowlist()
      const governanceAfter = await walletRegistryV2.governance()

      console.log(`Allowlist: ${newAllowlist}`)
      console.log(`Governance: ${governanceAfter}`)

      if (newAllowlist.toLowerCase() === Allowlist.address.toLowerCase()) {
        console.log()
        console.log("UPGRADE VERIFIED SUCCESSFULLY!")
        return true
      } else {
        console.error("VERIFICATION FAILED: Allowlist mismatch!")
        return false
      }
    } catch (error: any) {
      console.error("UPGRADE FAILED!")
      console.error("Error:", error.message)
      console.log()
      console.log("Try manual upgrade with cast:")
      console.log("```")
      console.log(`cast send ${proxyAdminAddress} \\`)
      console.log(`  "upgradeAndCall(address,address,bytes)" \\`)
      console.log(`  ${walletRegistryDeployment.address} \\`)
      console.log(`  ${newImplementation.address} \\`)
      console.log(`  "${initializeV2Data}" \\`)
      console.log(`  --rpc-url $CHAIN_API_URL \\`)
      console.log(`  --private-key <OWNER_KEY>`)
      console.log("```")
      return false
    }
  }
}

export default func

func.tags = ["UpgradeWalletRegistryV2"]
func.dependencies = ["Allowlist", "WalletRegistry", "EcdsaInactivity"]
func.id = "upgrade_wallet_registry_v2"

// Only run this script when explicitly requested
func.skip = async () => !process.env.UPGRADE_WALLET_REGISTRY_V2

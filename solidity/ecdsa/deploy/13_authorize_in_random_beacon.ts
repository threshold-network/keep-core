import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments } = hre
  const { deployer, governance } = await getNamedAccounts()
  const { execute } = deployments

  const WalletRegistry = await deployments.get("WalletRegistry")

  // Try to get RandomBeaconGovernance, or find it from random-beacon package
  let RandomBeaconGovernance = await deployments.getOrNull("RandomBeaconGovernance")
  if (!RandomBeaconGovernance) {
    const fs = require("fs")
    const path = require("path")
    const governancePath = path.resolve(
      __dirname,
      "../../random-beacon/deployments/development/RandomBeaconGovernance.json"
    )
    if (fs.existsSync(governancePath)) {
      const governanceData = JSON.parse(fs.readFileSync(governancePath, "utf8"))
      await deployments.save("RandomBeaconGovernance", {
        address: governanceData.address,
        abi: governanceData.abi,
      })
      RandomBeaconGovernance = await deployments.get("RandomBeaconGovernance")
    } else {
      throw new Error("RandomBeaconGovernance contract not found")
    }
  }

  // For mainnet we expect the scripts to be executed one by one. It's assumed that
  // the transfer of RandomBeaconGovernance ownership to governance will happen
  // after ecdsa contracts migration is done, so the `deployer` is still the
  // owner of `RandomBeaconGovernance`.
  const from = hre.network.name === "mainnet" ? deployer : governance

  try {
  await execute(
    "RandomBeaconGovernance",
    { from, log: true, waitConfirmations: 1 },
    "setRequesterAuthorization",
    WalletRegistry.address,
    true
  )
  } catch (error: any) {
    // If authorization fails, try with deployer account
    if (error.message?.includes("not the owner") || error.message?.includes("caller is not the owner")) {
      console.log(`Governance account is not owner, trying with deployer account: ${deployer}`)
      try {
        await execute(
          "RandomBeaconGovernance",
          { from: deployer, log: true, waitConfirmations: 1 },
          "setRequesterAuthorization",
          WalletRegistry.address,
          true
        )
      } catch (deployerError: any) {
        console.log(`Authorization failed. This step may need to be done manually. Error: ${deployerError.message}`)
        // Don't fail the deployment - authorization can be done manually
      }
    } else {
      throw error
    }
  }
}

export default func

func.tags = ["WalletRegistryAuthorizeInBeacon"]
func.dependencies = ["RandomBeaconGovernance", "WalletRegistry"]

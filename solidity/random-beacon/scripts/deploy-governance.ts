import { ethers } from "hardhat"
import hre from "hardhat"

async function main() {
  const [deployer] = await ethers.getSigners()
  console.log("Deployer:", deployer.address)

  const RandomBeacon = await hre.deployments.get("RandomBeacon")
  console.log("RandomBeacon:", RandomBeacon.address)

  const GOVERNANCE_DELAY = 604_800 // 1 week

  const RandomBeaconGovernanceFactory = await ethers.getContractFactory("RandomBeaconGovernance")
  const randomBeaconGovernance = await RandomBeaconGovernanceFactory.deploy(
    RandomBeacon.address,
    GOVERNANCE_DELAY
  )
  await randomBeaconGovernance.deployed()
  console.log("RandomBeaconGovernance deployed at:", randomBeaconGovernance.address)

  // Save it
  const abiJson = randomBeaconGovernance.interface.format(ethers.utils.FormatTypes.json)
  const deployment = {
    address: randomBeaconGovernance.address,
    abi: typeof abiJson === 'string' ? JSON.parse(abiJson) : abiJson,
  }
  await hre.deployments.save("RandomBeaconGovernance", deployment)
  console.log("Saved RandomBeaconGovernance deployment")
}

main().catch(console.error)


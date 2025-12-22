import { ethers } from "hardhat"
import hre from "hardhat"

async function main() {
  const [deployer] = await ethers.getSigners()
  console.log("Deployer:", deployer.address)

  const TokenStaking = await hre.deployments.get("TokenStaking")
  const RandomBeacon = await hre.deployments.get("RandomBeacon")
  
  // Check if already approved
  const tokenStakingContract = new ethers.Contract(
    TokenStaking.address,
    ["function applicationInfo(address) view returns (uint8 status, address panicButton)"],
    deployer
  )
  const appInfo = await tokenStakingContract.applicationInfo(RandomBeacon.address)
  if (appInfo.status === 1) { // ApplicationStatus.APPROVED = 1
    console.log("RandomBeacon is already approved. Skipping.")
    return
  }

  console.log("TokenStaking:", TokenStaking.address)
  console.log("RandomBeacon:", RandomBeacon.address)

  const tokenStaking = new ethers.Contract(
    TokenStaking.address,
    ["function approveApplication(address application)"],
    deployer
  )

  console.log("Approving RandomBeacon application...")
  const tx = await tokenStaking.approveApplication(RandomBeacon.address)
  await tx.wait()
  console.log("Approved! Tx:", tx.hash)
}

main().catch(console.error)


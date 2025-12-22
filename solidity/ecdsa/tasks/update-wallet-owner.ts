/* eslint-disable no-console */
import { task } from "hardhat/config"
import type { HardhatRuntimeEnvironment } from "hardhat/types"

task("update-wallet-owner", "Update Wallet Owner to a new address")
  .addParam("newOwner", "New Wallet Owner address")
  .setAction(async (args, hre: HardhatRuntimeEnvironment) => {
    const { newOwner } = args
    const { getNamedAccounts, ethers, helpers } = hre
    const { governance } = await getNamedAccounts()

    if (!ethers.utils.isAddress(newOwner)) {
      throw Error(`invalid address: ${newOwner}`)
    }

    const governanceContract = await helpers.contracts.getContract(
      "WalletRegistryGovernance"
    )
    const signer = await ethers.getSigner(governance)

    console.log(`Governance account: ${governance}`)
    console.log(`New wallet owner: ${newOwner}`)

    // Check if update already initiated
    try {
      const remaining = await governanceContract.getRemainingWalletOwnerUpdateTime()
      if (remaining.gt(0)) {
        console.log(`Update already initiated. Remaining time: ${remaining.toString()} seconds`)
        console.log("Trying to finalize...")
        try {
          const finalizeTx = await governanceContract.connect(signer).finalizeWalletOwnerUpdate()
          await finalizeTx.wait()
          console.log(`✅ Wallet owner updated! Transaction: ${finalizeTx.hash}`)
          return
        } catch (error: any) {
          if (error.message.includes("governance delay")) {
            console.log(`⏳ Need to wait ${remaining.toString()} more seconds before finalizing`)
            return
          }
          throw error
        }
      }
    } catch (error: any) {
      // Update not initiated yet, proceed to begin
      if (!error.message.includes("Change not initiated")) {
        throw error
      }
    }

    // Begin update
    console.log("Beginning wallet owner update...")
    const beginTx = await governanceContract
      .connect(signer)
      .beginWalletOwnerUpdate(newOwner)
    await beginTx.wait()
    console.log(`✅ Update initiated! Transaction: ${beginTx.hash}`)

    // Get governance delay
    const delay = await governanceContract.governanceDelay()
    console.log(`Governance delay: ${delay.toString()} seconds`)

    // Try to finalize immediately (might work if delay is 0 or very short)
    if (delay.eq(0)) {
      console.log("Delay is 0, finalizing immediately...")
      const finalizeTx = await governanceContract.connect(signer).finalizeWalletOwnerUpdate()
      await finalizeTx.wait()
      console.log(`✅ Wallet owner updated! Transaction: ${finalizeTx.hash}`)
    } else {
      console.log(`⏳ Need to wait ${delay.toString()} seconds before finalizing`)
      console.log("Run this command to finalize:")
      console.log(`  npx hardhat finalize-wallet-owner-update --network development`)
    }
  })

task("finalize-wallet-owner-update", "Finalize Wallet Owner update")
  .setAction(async (args, hre: HardhatRuntimeEnvironment) => {
    const { getNamedAccounts, ethers, helpers } = hre
    const { governance } = await getNamedAccounts()

    const governanceContract = await helpers.contracts.getContract(
      "WalletRegistryGovernance"
    )
    const signer = await ethers.getSigner(governance)

    console.log("Checking remaining time...")
    const remaining = await governanceContract.getRemainingWalletOwnerUpdateTime()
    
    if (remaining.gt(0)) {
      console.log(`⏳ Need to wait ${remaining.toString()} more seconds`)
      return
    }

    console.log("Finalizing wallet owner update...")
    const tx = await governanceContract.connect(signer).finalizeWalletOwnerUpdate()
    await tx.wait()
    console.log(`✅ Wallet owner updated! Transaction: ${tx.hash}`)
  })

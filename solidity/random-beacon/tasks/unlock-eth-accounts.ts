import { task } from "hardhat/config"

import type { HttpNetworkConfig } from "hardhat/types"

task("unlock-accounts", "Unlock ethereum accounts").setAction(
  async (args, hre) => {
    const { ethers } = hre

    if (hre.network.name === "development") {
      const password = process.env.KEEP_ETHEREUM_PASSWORD || "password"

      const provider = new ethers.providers.JsonRpcProvider(
        (hre.network.config as HttpNetworkConfig).url
      )
      const accounts = await provider.listAccounts()

      console.log(`Total accounts: ${accounts.length}`)
      console.log("---------------------------------")

      if (accounts.length === 0) {
        console.log("No accounts found. Make sure Geth is running with --unlock flag.")
        return
      }

      // Check if personal_unlockAccount is available (Geth < 1.16)
      let personalNamespaceAvailable = false
      try {
        await provider.send("personal_listAccounts", [])
        personalNamespaceAvailable = true
      } catch (error: any) {
        // If error code is -32601, the method doesn't exist (Geth 1.16+)
        if (error.code === -32601 || error.error?.code === -32601) {
          personalNamespaceAvailable = false
        } else {
          // Other errors, assume it's available but failed for another reason
          personalNamespaceAvailable = true
        }
      }

      if (!personalNamespaceAvailable) {
        console.log("\nGeth 1.16+ detected: personal namespace is deprecated.")
        console.log("Accounts should be unlocked via --unlock flag when starting Geth.")
        console.log("Skipping RPC unlock (accounts are already unlocked via --unlock).")
        console.log(`\nFound ${accounts.length} accounts available for use:`)
        for (let i = 0; i < accounts.length; i++) {
          console.log(`  Account ${i}: ${accounts[i]}`)
        }
        return
      }

      // For older Geth versions, unlock via RPC
      for (let i = 0; i < accounts.length; i++) {
        const account = accounts[i]

        try {
          console.log(`\nUnlocking account: ${account}`)
          // An explicit duration of zero seconds unlocks the key until geth exits.
          await provider.send("personal_unlockAccount", [
            account.toLowerCase(),
            password,
            0,
          ])
          console.log("Account unlocked!")
        } catch (error) {
          console.log(`\nAccount: ${account} not unlocked!`)
          console.error(error)
        }
        console.log("\n---------------------------------")
      }
    }
  }
)

import { task } from "hardhat/config"

import type { HttpNetworkConfig } from "hardhat/types"

task("unlock-accounts", "Unlock ethereum accounts").setAction(
  async (args, hre) => {
    const { ethers } = hre

    if (hre.network.name === "development") {
      const password = process.env.KEEP_ETHEREUM_PASSWORD || "password"

      const provider = new ethers.JsonRpcProvider(
        (hre.network.config as HttpNetworkConfig).url,
      )
      const accounts = await provider.listAccounts()

      console.log(`Total accounts: ${accounts.length}`)
      console.log("---------------------------------")

      for (let i = 0; i < accounts.length; i++) {
        const account = await accounts[i].getAddress()

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
  },
)

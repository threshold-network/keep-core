import { task } from "hardhat/config"

import type { HardhatRuntimeEnvironment } from "hardhat/types"

task("genesis", "Triggers the Random Beacon genesis").setAction(
  async (args, hre) => {
    await genesis(hre)
  },
)

async function genesis(hre: HardhatRuntimeEnvironment) {
  const { helpers } = hre
  const { governance } = await helpers.signers.getNamedSigners()

  const randomBeacon = await helpers.contracts.getContract("RandomBeacon")

  const genesisTx = await randomBeacon.connect(governance).genesis()
  await genesisTx.wait()

  console.log("Genesis was triggered successfully")
}

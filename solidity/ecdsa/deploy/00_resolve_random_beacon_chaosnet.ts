import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments, helpers } = hre
  const { log } = deployments

  const RandomBeaconChaosnet = await deployments.getOrNull(
    "RandomBeaconChaosnet"
  )

  if (
    RandomBeaconChaosnet &&
    helpers.address.isValid(RandomBeaconChaosnet.address)
  ) {
    log(
      `using existing RandomBeaconChaosnet at ${RandomBeaconChaosnet.address}`
    )
  } else {
    // Try to find it from the random-beacon package directly
    const fs = require("fs")
    const path = require("path")
    const randomBeaconPath = path.resolve(
      __dirname,
      "../../random-beacon/deployments/development/RandomBeaconChaosnet.json"
    )
    if (fs.existsSync(randomBeaconPath)) {
      const chaosnetData = JSON.parse(fs.readFileSync(randomBeaconPath, "utf8"))
      log(
        `using RandomBeaconChaosnet from random-beacon package at ${chaosnetData.address}`
      )
      // Register it with deployments
      await deployments.save("RandomBeaconChaosnet", {
        address: chaosnetData.address,
        abi: chaosnetData.abi,
      })
    } else {
      throw new Error("deployed RandomBeaconChaosnet contract not found")
    }
  }
}

export default func

func.tags = ["RandomBeaconChaosnet"]

func.skip = async (hre: HardhatRuntimeEnvironment): Promise<boolean> =>
  !hre.network.tags.useRandomBeaconChaosnet

import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments, helpers } = hre
  const { log } = deployments

  const ReimbursementPool = await deployments.getOrNull("ReimbursementPool")

  if (ReimbursementPool && helpers.address.isValid(ReimbursementPool.address)) {
    log(`using existing ReimbursementPool at ${ReimbursementPool.address}`)
  } else {
    // Try to find it from the random-beacon package directly
    const fs = require("fs")
    const path = require("path")
    const reimbursementPoolPath = path.resolve(
      __dirname,
      "../../random-beacon/deployments/development/ReimbursementPool.json"
    )
    if (fs.existsSync(reimbursementPoolPath)) {
      const poolData = JSON.parse(fs.readFileSync(reimbursementPoolPath, "utf8"))
      log(`using ReimbursementPool from random-beacon package at ${poolData.address}`)
      await deployments.save("ReimbursementPool", {
        address: poolData.address,
        abi: poolData.abi,
      })
    } else {
      throw new Error("deployed ReimbursementPool contract not found")
    }
  }
}

export default func

func.tags = ["ReimbursementPool"]

import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments, helpers } = hre
  const { log } = deployments

  const ReimbursementPool = await deployments.getOrNull("ReimbursementPool")

  if (ReimbursementPool && helpers.address.isValid(ReimbursementPool.address)) {
    log(`using existing ReimbursementPool at ${ReimbursementPool.address}`)
    return
  }

  if (hre.network.name === "hardhat" || hre.network.name === "development") {
    throw new Error(
      `ReimbursementPool not found on "${hre.network.name}". On local networks it is expected to come from the ` +
        "random-beacon external deploys — set USE_EXTERNAL_DEPLOY=true (the package.json test/deploy:test scripts " +
        "already do this) or pre-deploy ReimbursementPool yourself before running this script."
    )
  }
  throw new Error(
    `ReimbursementPool not found on "${hre.network.name}". For Sepolia, the committed ` +
      "deployments/sepolia/ReimbursementPool.json is the source of truth. For mainnet, ./external/mainnet must " +
      "contain ReimbursementPool.json."
  )
}

export default func

func.tags = ["ReimbursementPool"]

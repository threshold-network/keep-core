import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async function (hre: HardhatRuntimeEnvironment) {
  const { deployments, helpers } = hre
  const { log } = deployments

  const TokenStaking = await deployments.getOrNull("TokenStaking")

  if (TokenStaking && helpers.address.isValid(TokenStaking.address)) {
    log(`using existing TokenStaking at ${TokenStaking.address}`)
    return
  }

  if (hre.network.name === "hardhat" || hre.network.name === "development") {
    throw new Error(
      `TokenStaking not found on "${hre.network.name}". On local networks the contract is expected to come from ` +
        `@threshold-network/solidity-contracts external deploys — set USE_EXTERNAL_DEPLOY=true (the package.json test/deploy:test ` +
        `scripts already do this) or pre-deploy TokenStaking yourself before running this script.`
    )
  }
  throw new Error(
    `TokenStaking not found on "${hre.network.name}". For Sepolia, the committed deployments/sepolia/TokenStaking.json ` +
      `is the source of truth (external.deployments.sepolia is empty by design). For mainnet, ./external/mainnet must ` +
      `contain TokenStaking.json.`
  )
}

export default func

func.tags = ["TokenStaking"]

import verifyOnEtherscanOrContinue from "../deploy-utils/etherscanVerification"
import verifyOnTenderlyOrContinue from "../deploy-utils/tenderlyVerification"

import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers, ethers } = hre
  const { deployer } = await getNamedAccounts()

  const WalletRegistry = await deployments.get("WalletRegistry")

  const currentGovernance = await deployments.read(
    "WalletRegistry",
    "governance"
  )
  if (
    currentGovernance !== "0x0000000000000000000000000000000000000000" &&
    !helpers.address.equal(currentGovernance, deployer)
  ) {
    const artifact = await deployments.getArtifact("WalletRegistryGovernance")
    const liveGovernance = await ethers.getContractAt(
      artifact.abi,
      currentGovernance
    )
    const linkedRegistry = await liveGovernance.walletRegistry()
    if (!helpers.address.equal(linkedRegistry, WalletRegistry.address)) {
      throw new Error(
        `WalletRegistryGovernance at ${currentGovernance} points to ${linkedRegistry}, expected ${WalletRegistry.address}`
      )
    }
    const existing = await deployments.getOrNull("WalletRegistryGovernance")
    if (
      !existing ||
      !helpers.address.equal(existing.address, currentGovernance)
    ) {
      await deployments.save("WalletRegistryGovernance", {
        address: currentGovernance,
        abi: artifact.abi,
      })
    }
    deployments.log(
      `using live WalletRegistryGovernance at ${currentGovernance}`
    )
    return
  }

  // 60 seconds for Sepolia. 1 week otherwise.
  const GOVERNANCE_DELAY = hre.network.name === "sepolia" ? 60 : 604800

  const WalletRegistryGovernance = await deployments.deploy(
    "WalletRegistryGovernance",
    {
      from: deployer,
      skipIfAlreadyDeployed: true,
      args: [WalletRegistry.address, GOVERNANCE_DELAY],
      log: true,
      waitConfirmations: 1,
    }
  )

  if (
    hre.network.tags.etherscan &&
    process.env.DISABLE_HARDHAT_VERIFY !== "true"
  ) {
    await verifyOnEtherscanOrContinue(hre, () =>
      helpers.etherscan.verify(WalletRegistryGovernance)
    )
  }

  if (hre.network.tags.tenderly) {
    await verifyOnTenderlyOrContinue(hre, () =>
      hre.tenderly.verify({
        name: "WalletRegistryGovernance",
        address: WalletRegistryGovernance.address,
      })
    )
  }
}

export default func

func.tags = ["WalletRegistryGovernance"]
func.dependencies = ["WalletRegistry"]

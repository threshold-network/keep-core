import verifyOnEtherscanOrContinue from "./etherscanVerification"
import verifyOnTenderlyOrContinue from "./tenderlyVerification"

import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer } = await getNamedAccounts()

  const WalletRegistry = await deployments.get("WalletRegistry")

  // 60 seconds for Sepolia. 1 week otherwise.
  const GOVERNANCE_DELAY = hre.network.name === "sepolia" ? 60 : 604800

  const WalletRegistryGovernance = await deployments.deploy(
    "WalletRegistryGovernance",
    {
      from: deployer,
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

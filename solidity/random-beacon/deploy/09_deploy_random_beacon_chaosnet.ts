import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction, DeployOptions } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments, helpers } = hre
  const { deployer } = await getNamedAccounts()

  const deployOptions: DeployOptions = {
    from: deployer,
    log: true,
    waitConfirmations: 1,
  }

  const RandomBeaconChaosnet = await deployments.deploy(
    "RandomBeaconChaosnet",
    {
      ...deployOptions,
    },
  )

  if (hre.network.tags.etherscan) {
    if (RandomBeaconChaosnet.transactionHash) {
      await hre.ethers.provider.waitForTransaction(
        RandomBeaconChaosnet.transactionHash,
        2,
        300000,
      )
    }
    await helpers.etherscan.verify(RandomBeaconChaosnet)
  }

  if (hre.network.tags.tenderly) {
    await hre.tenderly.verify({
      name: "RandomBeaconChaosnet",
      address: RandomBeaconChaosnet.address,
    })
  }
}

export default func

func.tags = ["RandomBeaconChaosnet"]

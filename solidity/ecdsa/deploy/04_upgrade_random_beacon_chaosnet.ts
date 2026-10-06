import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments } = hre
  const { deployer } = await getNamedAccounts()
  const { execute } = deployments

  const RandomBeaconChaosnet = await deployments.get("RandomBeaconChaosnet")

  // Upgrade the random beacon smart contract in `WalletRegistry` to
  // `RandomBeaconChaosnet`. This is a temporary solution to enable usage of
  // `WalletRegistry` before the random beacon functionalities in the client
  // are ready.
  await execute(
    "WalletRegistry",
    { from: deployer, log: true, waitConfirmations: 1 },
    "upgradeRandomBeacon",
    RandomBeaconChaosnet.address,
  )
}

export default func

func.tags = ["UpgradeRandomBeaconChaosnet"]
func.dependencies = ["RandomBeaconChaosnet", "WalletRegistry"]

func.skip = async (hre: HardhatRuntimeEnvironment): Promise<boolean> =>
  !hre.network.tags.useRandomBeaconChaosnet

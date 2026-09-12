import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { getNamedAccounts, deployments } = hre
  const { deployer, governance } = await getNamedAccounts()
  const { execute } = deployments

  const WalletRegistry = await deployments.get("WalletRegistry")

  if (
    await deployments.read(
      "RandomBeacon",
      "authorizedRequesters",
      WalletRegistry.address
    )
  ) {
    return
  }
  const RandomBeaconGovernance = await deployments.get("RandomBeaconGovernance")
  const currentGovernance = await deployments.read("RandomBeacon", "governance")
  if (
    currentGovernance.toLowerCase() !==
    RandomBeaconGovernance.address.toLowerCase()
  ) {
    deployments.log(
      `RandomBeacon governance is ${currentGovernance}; skipping requester authorization through inactive RandomBeaconGovernance ${RandomBeaconGovernance.address}`
    )
    return
  }

  // For mainnet we expect the scripts to be executed one by one. It's assumed that
  // the transfer of RandomBeaconGovernance ownership to governance will happen
  // after ecdsa contracts migration is done, so the `deployer` is still the
  // owner of `RandomBeaconGovernance`.
  const from = hre.network.name === "mainnet" ? deployer : governance

  await execute(
    "RandomBeaconGovernance",
    { from, log: true, waitConfirmations: 1 },
    "setRequesterAuthorization",
    WalletRegistry.address,
    true
  )
}

export default func

func.tags = ["WalletRegistryAuthorizeInBeacon"]
func.dependencies = ["RandomBeaconGovernance", "WalletRegistry"]

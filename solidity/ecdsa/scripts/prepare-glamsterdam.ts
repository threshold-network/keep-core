/* eslint-disable import/no-extraneous-dependencies, no-console */
import { deployments, ethers, network } from "hardhat"

// Read-only: explicit, independently benchmarked inputs are required. No defaults.
function benchmarkedGas(name: string): string | undefined {
  const value = process.env[name]
  if (value === undefined) return undefined
  if (!/^[1-9][0-9]*$/.test(value)) {
    throw new Error(
      `${name} must be a positive integer from a full DKG benchmark`
    )
  }
  if (ethers.BigNumber.from(value).gt(ethers.constants.MaxUint256)) {
    throw new Error(`${name} exceeds uint256`)
  }
  return value
}

async function main() {
  const beaconGas = benchmarkedGas("GLAMSTERDAM_BEACON_DKG_SUBMISSION_GAS")
  const registryGas = benchmarkedGas("GLAMSTERDAM_WALLET_DKG_SUBMISSION_GAS")
  const blockNumber = await ethers.provider.getBlockNumber()
  const atBlock = { blockTag: blockNumber }
  const beaconDeployment = await deployments.get("RandomBeacon")
  const registryDeployment = await deployments.get("WalletRegistry")
  const beacon = new ethers.Contract(
    beaconDeployment.address,
    [
      "function governance() view returns (address)",
      "function relayEntryParameters() view returns (uint256,uint256,uint256)",
      "function gasParameters() view returns (uint256,uint256,uint256,uint256)",
    ],
    ethers.provider
  )
  const registry = new ethers.Contract(
    registryDeployment.address,
    [
      "function governance() view returns (address)",
      "function randomBeacon() view returns (address)",
      "function gasParameters() view returns (uint256,uint256,uint256,uint256,uint256)",
    ],
    ethers.provider
  )
  const governanceAbi = [
    "function owner() view returns (address)",
    "function governanceDelay() view returns (uint256)",
    "function beginDkgResultSubmissionGasUpdate(uint256)",
    "function finalizeDkgResultSubmissionGasUpdate()",
  ]
  const beaconGovernance = new ethers.Contract(
    await beacon.governance(atBlock),
    governanceAbi,
    ethers.provider
  )
  const registryGovernance = new ethers.Contract(
    await registry.governance(atBlock),
    governanceAbi,
    ethers.provider
  )
  const updates = [
    { target: beaconGovernance, value: beaconGas },
    { target: registryGovernance, value: registryGas },
  ].filter(({ value }) => value !== undefined)
  const call = (
    target: typeof beaconGovernance,
    method: string,
    args: string[] = []
  ) => ({
    to: target.address,
    value: "0",
    data: target.interface.encodeFunctionData(method, args),
    method,
  })
  console.log(
    JSON.stringify(
      {
        network: network.name,
        chainId: (await ethers.provider.getNetwork()).chainId,
        blockNumber,
        walletRegistry: registry.address,
        walletRegistryRandomBeacon: await registry.randomBeacon(atBlock),
        cappedRandomBeacon: beacon.address,
        // The deployed RandomBeacon artifact is not necessarily the registry's
        // actual provider. Never infer callback wiring from its parameters.
        callbackUpdateGenerated: false,
        currentParameters: {
          beaconRelay: (await beacon.relayEntryParameters(atBlock)).map(String),
          beaconGas: (await beacon.gasParameters(atBlock)).map(String),
          registryGas: (await registry.gasParameters(atBlock)).map(String),
        },
        owners: {
          randomBeacon: await beaconGovernance.owner(atBlock),
          walletRegistry: await registryGovernance.owner(atBlock),
        },
        governanceDelays: {
          randomBeacon: (
            await beaconGovernance.governanceDelay(atBlock)
          ).toString(),
          walletRegistry: (
            await registryGovernance.governanceDelay(atBlock)
          ).toString(),
        },
        begin: updates.map(({ target, value }) =>
          call(target, "beginDkgResultSubmissionGasUpdate", [value as string])
        ),
        // Submit only after each on-chain pending value and delay are checked.
        finalize: updates.map(({ target }) =>
          call(target, "finalizeDkgResultSubmissionGasUpdate")
        ),
      },
      null,
      2
    )
  )
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})

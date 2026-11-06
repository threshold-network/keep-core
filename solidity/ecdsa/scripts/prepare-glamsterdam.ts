/* eslint-disable import/no-extraneous-dependencies, no-console */
import { deployments, ethers, network } from "hardhat"

// Read-only: print governance calls for review; never submit a transaction.
async function main() {
  const beaconDeployment = await deployments.get("RandomBeacon")
  const registryDeployment = await deployments.get("WalletRegistry")
  const beacon = new ethers.Contract(
    beaconDeployment.address,
    ["function governance() view returns (address)"],
    ethers.provider
  )
  const registry = new ethers.Contract(
    registryDeployment.address,
    ["function governance() view returns (address)"],
    ethers.provider
  )
  const governanceAbi = [
    "function owner() view returns (address)",
    "function governanceDelay() view returns (uint256)",
    "function beginCallbackGasLimitUpdate(uint256)",
    "function finalizeCallbackGasLimitUpdate()",
    "function beginDkgResultSubmissionGasUpdate(uint256)",
    "function finalizeDkgResultSubmissionGasUpdate()",
  ]
  const beaconGovernance = new ethers.Contract(
    await beacon.governance(),
    governanceAbi,
    ethers.provider
  )
  const registryGovernance = new ethers.Contract(
    await registry.governance(),
    governanceAbi,
    ethers.provider
  )
  const call = (
    target: typeof beaconGovernance,
    method: string,
    args: number[] = []
  ) => ({
    to: target.address,
    value: "0",
    data: target.interface.encodeFunctionData(method, args),
    method,
  })
  const updates = [
    { target: beaconGovernance, parameter: "CallbackGasLimit", value: 300_000 },
    {
      target: beaconGovernance,
      parameter: "DkgResultSubmissionGas",
      value: 450_000,
    },
    {
      target: registryGovernance,
      parameter: "DkgResultSubmissionGas",
      value: 500_000,
    },
  ]
  console.log(
    JSON.stringify(
      {
        network: network.name,
        chainId: (await ethers.provider.getNetwork()).chainId,
        blockNumber: await ethers.provider.getBlockNumber(),
        provisional: true,
        owners: {
          randomBeacon: await beaconGovernance.owner(),
          walletRegistry: await registryGovernance.owner(),
        },
        governanceDelays: {
          randomBeacon: (await beaconGovernance.governanceDelay()).toString(),
          walletRegistry: (
            await registryGovernance.governanceDelay()
          ).toString(),
        },
        begin: updates.map(({ target, parameter, value }) =>
          call(target, `begin${parameter}Update`, [value])
        ),
        // Submit only after each on-chain pending value and delay are checked.
        finalize: updates.map(({ target, parameter }) =>
          call(target, `finalize${parameter}Update`)
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

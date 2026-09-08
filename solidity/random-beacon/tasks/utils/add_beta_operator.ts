/* eslint-disable no-console */
import type { HardhatRuntimeEnvironment } from "hardhat/types"

// eslint-disable-next-line import/prefer-default-export
export async function addBetaOperator(
  hre: HardhatRuntimeEnvironment,
  sortitionPoolDeploymentName: string,
  operator: string,
): Promise<void> {
  const { ethers, helpers } = hre
  const sortitionPool = await helpers.contracts.getContract(
    sortitionPoolDeploymentName,
  )
  const chaosnetOwner = await sortitionPool.chaosnetOwner()

  console.log(`Adding ${operator} to the set of beta operators...`)
  await (
    await sortitionPool
      .connect(await ethers.getSigner(chaosnetOwner))
      .getFunction("addBetaOperators")([operator])
  ).wait()
}

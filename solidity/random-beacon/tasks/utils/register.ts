/* eslint-disable no-console */
import type { HardhatRuntimeEnvironment } from "hardhat/types"

// eslint-disable-next-line import/prefer-default-export
export async function register(
  hre: HardhatRuntimeEnvironment,
  deploymentName: string,
  provider: string,
  operator: string,
): Promise<void> {
  const { ethers, helpers } = hre

  const providerAddress = ethers.utils.getAddress(provider)
  const operatorAddress = ethers.utils.getAddress(operator)

  const application = await helpers.contracts.getContract(deploymentName)

  console.log(
    `Registering operator ${operatorAddress} in ${deploymentName} application (${application.address})`,
  )

  const currentProvider = ethers.utils.getAddress(
    await application.callStatic.operatorToStakingProvider(operatorAddress),
  )

  switch (currentProvider) {
    case providerAddress: {
      console.log(
        `Current staking provider for operator ${operatorAddress} is ${currentProvider}`,
      )
      return
    }
    case ethers.constants.AddressZero: {
      console.log(
        `Registering operator ${operatorAddress} for a staking provider ${providerAddress}...`,
      )

      await (
        await application
          .connect(await ethers.getSigner(providerAddress))
          .registerOperator(operatorAddress)
      ).wait()

      break
    }
    default: {
      throw new Error(
        `Operator [${operatorAddress}] has already been registered for another staking provider [${currentProvider}]`,
      )
    }
  }
}

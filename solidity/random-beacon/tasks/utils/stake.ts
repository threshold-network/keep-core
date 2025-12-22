/* eslint-disable no-console */
import type { BigNumberish, BigNumber } from "ethers"
import type { HardhatRuntimeEnvironment } from "hardhat/types"

export async function stake(
  hre: HardhatRuntimeEnvironment,
  owner: string,
  provider: string,
  amount: BigNumberish,
  beneficiary?: string,
  authorizer?: string
): Promise<void> {
  const { ethers, helpers } = hre
  const { to1e18, from1e18 } = helpers.number
  const ownerAddress = ethers.utils.getAddress(owner)
  const providerAddress = ethers.utils.getAddress(provider)
  const stakeAmount = to1e18(amount)

  // Beneficiary can equal to the owner if not set otherwise. This simplification
  // is used for development purposes.
  const beneficiaryAddress = beneficiary
    ? ethers.utils.getAddress(beneficiary)
    : ownerAddress

  // Authorizer can equal to the owner if not set otherwise. This simplification
  // is used for development purposes.
  const authorizerAddress = authorizer
    ? ethers.utils.getAddress(authorizer)
    : ownerAddress

  const staking = await helpers.contracts.getContract("TokenStaking")

  const { tStake: currentStake } = await staking.callStatic.stakes(
    providerAddress
  )

  console.log(
    `Current stake for ${providerAddress} is ${from1e18(currentStake)} T`
  )

  if (currentStake.eq(0)) {
    console.log(
      `Staking ${from1e18(
        stakeAmount
      )} T to the staking provider ${providerAddress}...`
    )

    const stakingWithSigner = staking.connect(await ethers.getSigner(ownerAddress))
    
    try {
      // Try to call stake function (available in ExtendedTokenStaking)
      await (await stakingWithSigner.stake(
          providerAddress,
          beneficiaryAddress,
          authorizerAddress,
          stakeAmount
      )).wait()
    } catch (error: any) {
      if (error.message?.includes('stake is not a function') || 
          error.message?.includes('is not a function')) {
        throw new Error(
          'TokenStaking contract does not have a stake function. ' +
          'For development, you need to deploy ExtendedTokenStaking (from TokenStakingTestSet.sol) ' +
          'instead of TokenStaking. The stake function is only available in ExtendedTokenStaking ' +
          'which is used for testing and development.'
        )
      }
      throw error
    }
  } else if (currentStake.lt(stakeAmount)) {
    const topUpAmount = stakeAmount.sub(currentStake)

    console.log(
      `Topping up ${from1e18(
        topUpAmount
      )} T to the staking provider ${providerAddress}...`
    )

    await (
      await staking
        .connect(await ethers.getSigner(ownerAddress))
        .topUp(providerAddress, topUpAmount)
    ).wait()
  }
}

export async function calculateTokensNeededForStake(
  hre: HardhatRuntimeEnvironment,
  provider: string,
  amount: BigNumberish
): Promise<BigNumber> {
  const { ethers, helpers } = hre
  const { to1e18, from1e18 } = helpers.number

  const stakeAmount = to1e18(amount)

  const staking = await helpers.contracts.getContract("TokenStaking")

  const { tStake: currentStake } = await staking.callStatic.stakes(provider)

  if (currentStake.lt(stakeAmount)) {
    return ethers.BigNumber.from(from1e18(stakeAmount.sub(currentStake)))
  }

  return ethers.constants.Zero
}

/* eslint-disable no-console */
import type { BigNumberish } from "ethers"
import type { HardhatRuntimeEnvironment } from "hardhat/types"

export async function stake(
  hre: HardhatRuntimeEnvironment,
  owner: string,
  provider: string,
  amount: BigNumberish,
  beneficiary?: string,
  authorizer?: string,
): Promise<void> {
  const { ethers, helpers } = hre
  const { to1e18, from1e18 } = helpers.number
  const ownerAddress = ethers.getAddress(owner)
  const providerAddress = ethers.getAddress(provider)
  const stakeAmount = to1e18(amount)

  // Beneficiary can equal to the owner if not set otherwise. This simplification
  // is used for development purposes.
  const beneficiaryAddress = beneficiary
    ? ethers.getAddress(beneficiary)
    : ownerAddress

  // Authorizer can equal to the owner if not set otherwise. This simplification
  // is used for development purposes.
  const authorizerAddress = authorizer
    ? ethers.getAddress(authorizer)
    : ownerAddress

  const staking = await helpers.contracts.getContract("TokenStaking")

  const { tStake: currentStake }: { tStake: bigint } =
    await staking.stakes.staticCall(providerAddress)

  console.log(
    `Current stake for ${providerAddress} is ${from1e18(currentStake)} T`,
  )

  if (currentStake === 0n) {
    console.log(
      `Staking ${from1e18(
        stakeAmount,
      )} T to the staking provider ${providerAddress}...`,
    )

    await (
      await staking
        .connect(await ethers.getSigner(ownerAddress))
        .getFunction("stake")(
        providerAddress,
        beneficiaryAddress,
        authorizerAddress,
        stakeAmount,
      )
    ).wait()
  } else if (currentStake < stakeAmount) {
    const topUpAmount = stakeAmount - currentStake

    console.log(
      `Topping up ${from1e18(
        topUpAmount,
      )} T to the staking provider ${providerAddress}...`,
    )

    await (
      await staking
        .connect(await ethers.getSigner(ownerAddress))
        .getFunction("topUp")(providerAddress, topUpAmount)
    ).wait()
  }
}

export async function calculateTokensNeededForStake(
  hre: HardhatRuntimeEnvironment,
  provider: string,
  amount: BigNumberish,
): Promise<bigint> {
  const { ethers, helpers } = hre
  const { to1e18, from1e18 } = helpers.number

  const stakeAmount = to1e18(amount)

  const staking = await helpers.contracts.getContract("TokenStaking")

  const { tStake: currentStake }: { tStake: bigint } =
    await staking.stakes.staticCall(provider)

  if (currentStake < stakeAmount) {
    return BigInt(from1e18(stakeAmount - currentStake))
  }

  return 0n
}

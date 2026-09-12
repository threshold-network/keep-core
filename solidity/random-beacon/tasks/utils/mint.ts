/* eslint-disable no-console */
import type { BigNumberish, BigNumber } from "ethers"
import type { HardhatRuntimeEnvironment } from "hardhat/types"

// eslint-disable-next-line import/prefer-default-export
export async function mint(
  hre: HardhatRuntimeEnvironment,
  owner: string,
  amount: BigNumberish,
): Promise<void> {
  const { ethers, helpers } = hre
  const { to1e18, from1e18 } = helpers.number
  const ownerAddress = ethers.utils.getAddress(owner)
  const stakeAmount = to1e18(amount)

  const t = await helpers.contracts.getContract("T")
  const staking = await helpers.contracts.getContract("TokenStaking")

  const tokenContractOwner = await t.owner()

  const currentBalance: BigNumber = await t.balanceOf(ownerAddress)

  console.log(
    `Account ${ownerAddress} balance is ${from1e18(currentBalance)} T`,
  )

  if (currentBalance.lt(stakeAmount)) {
    const mintAmount = stakeAmount.sub(currentBalance)

    console.log(`Minting ${from1e18(mintAmount)} T for ${ownerAddress}...`)

    await (
      await t
        .connect(await ethers.getSigner(tokenContractOwner))
        .mint(ownerAddress, mintAmount)
    ).wait()
  }

  const currentAllowance: BigNumber = await t.allowance(
    ownerAddress,
    staking.address,
  )

  console.log(
    `Account ${ownerAddress} allowance for ${staking.address} is ${from1e18(
      currentAllowance,
    )} T`,
  )

  if (currentAllowance.lt(stakeAmount)) {
    console.log(
      `Approving ${from1e18(stakeAmount)} T for ${staking.address}...`,
    )
    await (
      await t
        .connect(await ethers.getSigner(ownerAddress))
        .approve(staking.address, stakeAmount)
    ).wait()
  }
}

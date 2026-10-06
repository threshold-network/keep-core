/* eslint-disable no-console */
import type { BigNumberish } from "ethers"
import type { HardhatRuntimeEnvironment } from "hardhat/types"

// eslint-disable-next-line import/prefer-default-export
export async function mint(
  hre: HardhatRuntimeEnvironment,
  owner: string,
  amount: BigNumberish,
): Promise<void> {
  const { ethers, helpers } = hre
  const { to1e18, from1e18 } = helpers.number
  const ownerAddress = ethers.getAddress(owner)
  const stakeAmount = to1e18(amount)

  const t = await helpers.contracts.getContract("T")
  const staking = await helpers.contracts.getContract("TokenStaking")

  const tokenContractOwner = await t.owner()

  const currentBalance: bigint = await t.balanceOf(ownerAddress)

  console.log(
    `Account ${ownerAddress} balance is ${from1e18(currentBalance)} T`,
  )

  if (currentBalance < stakeAmount) {
    const mintAmount = stakeAmount - currentBalance

    console.log(`Minting ${from1e18(mintAmount)} T for ${ownerAddress}...`)

    await (
      await t
        .connect(await ethers.getSigner(tokenContractOwner))
        .getFunction("mint")(ownerAddress, mintAmount)
    ).wait()
  }

  const currentAllowance: bigint = await t.allowance(
    ownerAddress,
    await staking.getAddress(),
  )

  console.log(
    `Account ${ownerAddress} allowance for ${await staking.getAddress()} is ${from1e18(
      currentAllowance,
    )} T`,
  )

  if (currentAllowance < stakeAmount) {
    console.log(
      `Approving ${from1e18(stakeAmount)} T for ${await staking.getAddress()}...`,
    )
    await (
      await t
        .connect(await ethers.getSigner(ownerAddress))
        .getFunction("approve")(await staking.getAddress(), stakeAmount)
    ).wait()
  }
}

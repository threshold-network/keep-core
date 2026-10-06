/* eslint-disable no-console */
import type { BigNumberish } from "ethers"
import type { HardhatRuntimeEnvironment } from "hardhat/types"

// eslint-disable-next-line import/prefer-default-export
export async function authorize(
  hre: HardhatRuntimeEnvironment,
  deploymentName: string, // TODO: Change to IApplication
  owner: string,
  provider: string,
  authorizer?: string,
  authorization?: BigNumberish,
): Promise<void> {
  const { ethers, helpers } = hre
  const ownerAddress = ethers.getAddress(owner)
  const providerAddress = ethers.getAddress(provider)

  const application = await helpers.contracts.getContract(deploymentName)

  console.log(
    `Authorizing provider's ${providerAddress} stake in ${deploymentName} application (${await application.getAddress()})`,
  )

  // Authorizer can equal to the owner if not set otherwise. This simplification
  // is used for development purposes.
  const authorizerAddress = authorizer
    ? ethers.getAddress(authorizer)
    : ownerAddress

  const { to1e18, from1e18 } = helpers.number
  const staking = await helpers.contracts.getContract("TokenStaking")

  const authorizationBN: bigint = authorization
    ? to1e18(authorization)
    : await application.minimumAuthorization()

  const currentAuthorization: bigint = await staking.authorizedStake(
    providerAddress,
    await application.getAddress(),
  )

  if (currentAuthorization >= authorizationBN) {
    console.log(
      `Authorized stake is already ${from1e18(currentAuthorization)} T`,
    )
    return
  }

  const increaseAmount: bigint = authorizationBN - currentAuthorization

  console.log(
    `Increasing authorization by ${from1e18(increaseAmount)} T to ${from1e18(
      authorizationBN,
    )} T...`,
  )

  await (
    await staking
      .connect(await ethers.getSigner(authorizerAddress))
      .getFunction("increaseAuthorization")(
      providerAddress,
      await application.getAddress(),
      increaseAmount,
    )
  ).wait()
}

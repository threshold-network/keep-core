/* eslint-disable no-console */
import type { BigNumber, BigNumberish } from "ethers"
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
  const ownerAddress = ethers.utils.getAddress(owner)
  const providerAddress = ethers.utils.getAddress(provider)

  const application = await helpers.contracts.getContract(deploymentName)

  console.log(
    `Authorizing provider's ${providerAddress} stake in ${deploymentName} application (${application.address})`,
  )

  // Authorizer can equal to the owner if not set otherwise. This simplification
  // is used for development purposes.
  const authorizerAddress = authorizer
    ? ethers.utils.getAddress(authorizer)
    : ownerAddress

  const { to1e18, from1e18 } = helpers.number
  const staking = await helpers.contracts.getContract("TokenStaking")

  const authorizationBN: BigNumber = authorization
    ? to1e18(authorization)
    : await application.minimumAuthorization()

  const currentAuthorization = await staking.authorizedStake(
    providerAddress,
    application.address,
  )

  if (currentAuthorization.gte(authorizationBN)) {
    console.log(
      `Authorized stake is already ${from1e18(currentAuthorization)} T`,
    )
    return
  }

  const increaseAmount: BigNumber = authorizationBN.sub(currentAuthorization)

  console.log(
    `Increasing authorization by ${from1e18(increaseAmount)} T to ${from1e18(
      authorizationBN,
    )} T...`,
  )

  await (
    await staking
      .connect(await ethers.getSigner(authorizerAddress))
      .increaseAuthorization(
        providerAddress,
        application.address,
        increaseAmount,
      )
  ).wait()
}

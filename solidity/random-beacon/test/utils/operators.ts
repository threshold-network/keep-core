import { ethers, helpers } from "hardhat"

import { params } from "../fixtures"
import { testConfig } from "../../hardhat.config"

import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { BigNumber, BigNumberish, Contract } from "ethers"
import type {
  RandomBeacon,
  RandomBeaconStub,
  SortitionPool,
  T,
  TokenStaking,
} from "../../typechain"

/** Minimal ABI for TokenStaking methods omitted from the generated Typechain ABI. */
const legacyTokenStakingIface = new ethers.utils.Interface([
  "function stake(address,address,address,uint96)",
  "function increaseAuthorization(address,address,uint96)",
  "function approveApplication(address)",
  "function processSlashing(uint256)",
  "function requestAuthorizationDecrease(address,address,uint96)",
])

export function legacyTokenStakingAt(
  staking: Pick<TokenStaking, "address">,
  signer: SignerWithAddress
): Contract {
  return new ethers.Contract(staking.address, legacyTokenStakingIface, signer)
}

export type OperatorID = number
export type Operator = { id: OperatorID; signer: SignerWithAddress }

export async function registerOperators(
  randomBeacon: RandomBeacon,
  t: T,
  numberOfOperators = testConfig.operatorsCount,
  unnamedSignersOffset = testConfig.nonStakingAccountsCount,
  stakeAmount: BigNumber = params.minimumAuthorization
): Promise<Operator[]> {
  const operators: Operator[] = []

  const sortitionPool: SortitionPool = await ethers.getContractAt(
    "SortitionPool",
    await randomBeacon.sortitionPool()
  )

  const staking: TokenStaking = await ethers.getContractAt(
    "TokenStaking",
    await randomBeacon.staking()
  )

  const signers = (await helpers.signers.getUnnamedSigners()).slice(
    unnamedSignersOffset
  )

  // We use unique accounts for each staking role for each operator.
  if (signers.length < numberOfOperators * 5) {
    throw new Error(
      "not enough unnamed signers; update hardhat network's configuration account count"
    )
  }

  for (let i = 0; i < numberOfOperators; i++) {
    const owner: SignerWithAddress = signers[i]
    const stakingProvider: SignerWithAddress =
      signers[1 * numberOfOperators + i]
    const operator: SignerWithAddress = signers[2 * numberOfOperators + i]
    const beneficiary: SignerWithAddress = signers[3 * numberOfOperators + i]
    const authorizer: SignerWithAddress = signers[4 * numberOfOperators + i]

    await stake(
      t,
      staking,
      randomBeacon,
      owner,
      stakingProvider,
      stakeAmount,
      beneficiary,
      authorizer
    )

    await randomBeacon
      .connect(stakingProvider)
      .registerOperator(operator.address)

    await randomBeacon.connect(operator).joinSortitionPool()

    const id = await sortitionPool.getOperatorID(operator.address)

    operators.push({ id, signer: operator })
  }

  return operators
}

export async function stake(
  t: T,
  staking: TokenStaking,
  randomBeacon: RandomBeacon | RandomBeaconStub,
  owner: SignerWithAddress,
  stakingProvider: SignerWithAddress,
  stakeAmount: BigNumberish,
  beneficiary = stakingProvider,
  authorizer = stakingProvider
): Promise<void> {
  const { deployer } = await helpers.signers.getNamedSigners()

  await t.connect(deployer).mint(owner.address, stakeAmount)
  await t.connect(owner).approve(staking.address, stakeAmount)

  await legacyTokenStakingAt(staking, owner).stake(
    stakingProvider.address,
    beneficiary.address,
    authorizer.address,
    stakeAmount
  )

  await legacyTokenStakingAt(staking, authorizer).increaseAuthorization(
    stakingProvider.address,
    randomBeacon.address,
    stakeAmount
  )
}

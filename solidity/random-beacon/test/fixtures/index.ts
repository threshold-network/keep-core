import { ethers, helpers, deployments } from "hardhat"

import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { Contract } from "ethers"
import type {
  SortitionPool,
  BeaconDkgValidator as DKGValidator,
  RandomBeaconStub,
  TokenStaking,
  RandomBeaconGovernance,
  T,
  ReimbursementPool,
} from "../../typechain"

const { to1e18 } = helpers.number

export const constants = {
  groupSize: 64,
  groupThreshold: 33,
  offchainDkgTime: 72, // 5 * (1 + 5) + 2 * (1 + 10) + 20
  poolWeightDivisor: to1e18(1),
  tokenStakingNotificationReward: to1e18(10000), // 10k T
}

export const dkgState = {
  IDLE: 0,
  AWAITING_SEED: 1,
  KEY_GENERATION: 2,
  AWAITING_RESULT: 3,
  CHALLENGE: 4,
}

export const params = {
  governanceDelay: 604_800, // 1 week
  relayEntrySoftTimeout: 35,
  relayEntryHardTimeout: 100,
  callbackGasLimit: 200_000,
  groupCreationFrequency: 10,
  groupLifetime: 5761, // 1 day in blocks assuming 15s block time
  dkgResultChallengePeriodLength: 100,
  dkgResultChallengeExtraGas: 50_000,
  dkgResultSubmissionTimeout: 30,
  dkgSubmitterPrecedencePeriodLength: 5,
  sortitionPoolRewardsBanDuration: 1_209_600, // 2 weeks
  relayEntrySubmissionFailureSlashingAmount: to1e18(1000),
  maliciousDkgResultSlashingAmount: to1e18(50_000),
  relayEntryTimeoutNotificationRewardMultiplier: 40,
  unauthorizedSigningNotificationRewardMultiplier: 50,
  dkgMaliciousResultNotificationRewardMultiplier: 100,
  unauthorizedSigningSlashingAmount: to1e18(100_000),
  minimumAuthorization: to1e18(200_000),
  authorizationDecreaseDelay: 403_200,
  authorizationDecreaseChangePeriod: 403_200,
  reimbursementPoolStaticGas: 40_800,
  reimbursementPoolMaxGasPrice: ethers.utils.parseUnits("500", "gwei"),
  dkgResultSubmissionGas: 237_650,
  dkgResultApprovalGasOffset: 41_500,
  notifyOperatorInactivityGasOffset: 54_500,
  relayEntrySubmissionGasOffset: 11_250,
}

export interface DeployedContracts {
  [key: string]: Contract
}

export async function blsDeployment(): Promise<DeployedContracts> {
  const BLS = await ethers.getContractFactory("BLS")
  const bls = await BLS.deploy()
  await bls.deployed()

  const contracts: DeployedContracts = { bls }

  return contracts
}

export async function randomBeaconDeployment(): Promise<DeployedContracts> {
  await deployments.fixture()
  const t: T = await helpers.contracts.getContract<T>("T")

  const staking: TokenStaking =
    await helpers.contracts.getContract<TokenStaking>("TokenStaking")

  const { deployer, chaosnetOwner } = await helpers.signers.getNamedSigners()

  const sortitionPool: SortitionPool = await helpers.contracts.getContract(
    "BeaconSortitionPool"
  )
  const randomBeaconGovernance: RandomBeaconGovernance =
    await helpers.contracts.getContract("RandomBeaconGovernance")

  const reimbursementPool: ReimbursementPool =
    await helpers.contracts.getContract("ReimbursementPool")

  await deployer.sendTransaction({
    to: reimbursementPool.address,
    value: ethers.utils.parseEther("100.0"), // Send 100.0 ETH
  })

  const randomBeacon: RandomBeaconStub = await helpers.contracts.getContract(
    "RandomBeacon"
  )

  await updateTokenStakingParams(t, staking, deployer)
  await setFixtureParameters(randomBeacon)

  await sortitionPool.connect(chaosnetOwner).deactivateChaosnet()

  const contracts: DeployedContracts = {
    sortitionPool,
    staking,
    randomBeacon,
    t,
    reimbursementPool,
    randomBeaconGovernance,
  }

  return contracts
}

async function updateTokenStakingParams(
  t: T,
  staking: TokenStaking,
  deployer: SignerWithAddress
) {
  // initialNotifierTreasury should be configured high enough to execute all the
  // slashing in test suites.
  const initialNotifierTreasury = to1e18(9_000_000) // 9MM T
  await t.connect(deployer).approve(staking.address, initialNotifierTreasury)

  // Compatibility: Threshold TokenStaking variant may not expose these methods.
  const stakingAsRecord = staking.connect(deployer) as unknown as Record<
    string,
    (...args: unknown[]) => Promise<unknown>
  >

  if (typeof stakingAsRecord.pushNotificationReward === "function") {
    await stakingAsRecord.pushNotificationReward(initialNotifierTreasury)
  }
  if (typeof stakingAsRecord.setNotificationReward === "function") {
    await stakingAsRecord.setNotificationReward(
      constants.tokenStakingNotificationReward
    )
  }
}

async function setFixtureParameters(randomBeacon: RandomBeaconStub) {
  const randomBeaconGovernance: RandomBeaconGovernance =
    await helpers.contracts.getContract("RandomBeaconGovernance")
  const { governance } = await helpers.signers.getNamedSigners()

  await randomBeaconGovernance
    .connect(governance)
    .beginMinimumAuthorizationUpdate(params.minimumAuthorization)
  await randomBeaconGovernance
    .connect(governance)
    .beginAuthorizationDecreaseDelayUpdate(params.authorizationDecreaseDelay)
  await randomBeaconGovernance
    .connect(governance)
    .beginAuthorizationDecreaseChangePeriodUpdate(
      params.authorizationDecreaseChangePeriod
    )

  await randomBeaconGovernance
    .connect(governance)
    .beginRelayEntrySoftTimeoutUpdate(params.relayEntrySoftTimeout)
  await randomBeaconGovernance
    .connect(governance)
    .beginRelayEntryHardTimeoutUpdate(params.relayEntryHardTimeout)
  await randomBeaconGovernance
    .connect(governance)
    .beginCallbackGasLimitUpdate(params.callbackGasLimit)

  await randomBeaconGovernance
    .connect(governance)
    .beginSortitionPoolRewardsBanDurationUpdate(
      params.sortitionPoolRewardsBanDuration
    )
  await randomBeaconGovernance
    .connect(governance)
    .beginRelayEntryTimeoutNotificationRewardMultiplierUpdate(
      params.relayEntryTimeoutNotificationRewardMultiplier
    )
  await randomBeaconGovernance
    .connect(governance)
    .beginUnauthorizedSigningNotificationRewardMultiplierUpdate(
      params.unauthorizedSigningNotificationRewardMultiplier
    )
  await randomBeaconGovernance
    .connect(governance)
    .beginDkgMaliciousResultNotificationRewardMultiplierUpdate(
      params.dkgMaliciousResultNotificationRewardMultiplier
    )

  await randomBeaconGovernance
    .connect(governance)
    .beginGroupCreationFrequencyUpdate(params.groupCreationFrequency)
  await randomBeaconGovernance
    .connect(governance)
    .beginGroupLifetimeUpdate(params.groupLifetime)
  await randomBeaconGovernance
    .connect(governance)
    .beginDkgResultChallengePeriodLengthUpdate(
      params.dkgResultChallengePeriodLength
    )
  await randomBeaconGovernance
    .connect(governance)
    .beginDkgResultSubmissionTimeoutUpdate(params.dkgResultSubmissionTimeout)
  await randomBeaconGovernance
    .connect(governance)
    .beginDkgSubmitterPrecedencePeriodLengthUpdate(
      params.dkgSubmitterPrecedencePeriodLength
    )

  await randomBeaconGovernance
    .connect(governance)
    .beginRelayEntrySubmissionFailureSlashingAmountUpdate(
      params.relayEntrySubmissionFailureSlashingAmount
    )
  await randomBeaconGovernance
    .connect(governance)
    .beginMaliciousDkgResultSlashingAmountUpdate(
      params.maliciousDkgResultSlashingAmount
    )
  await randomBeaconGovernance
    .connect(governance)
    .beginUnauthorizedSigningSlashingAmountUpdate(
      params.unauthorizedSigningSlashingAmount
    )

  await helpers.time.increaseTime(params.governanceDelay)

  await randomBeaconGovernance
    .connect(governance)
    .finalizeMinimumAuthorizationUpdate()
  await randomBeaconGovernance
    .connect(governance)
    .finalizeAuthorizationDecreaseDelayUpdate()
  await randomBeaconGovernance
    .connect(governance)
    .finalizeAuthorizationDecreaseChangePeriodUpdate()

  await randomBeaconGovernance
    .connect(governance)
    .finalizeRelayEntrySoftTimeoutUpdate()
  await randomBeaconGovernance
    .connect(governance)
    .finalizeRelayEntryHardTimeoutUpdate()
  await randomBeaconGovernance
    .connect(governance)
    .finalizeCallbackGasLimitUpdate()

  await randomBeaconGovernance
    .connect(governance)
    .finalizeSortitionPoolRewardsBanDurationUpdate()
  await randomBeaconGovernance
    .connect(governance)
    .finalizeRelayEntryTimeoutNotificationRewardMultiplierUpdate()
  await randomBeaconGovernance
    .connect(governance)
    .finalizeUnauthorizedSigningNotificationRewardMultiplierUpdate()
  await randomBeaconGovernance
    .connect(governance)
    .finalizeDkgMaliciousResultNotificationRewardMultiplierUpdate()

  await randomBeaconGovernance
    .connect(governance)
    .finalizeGroupCreationFrequencyUpdate()
  await randomBeaconGovernance.connect(governance).finalizeGroupLifetimeUpdate()
  await randomBeaconGovernance
    .connect(governance)
    .finalizeDkgResultChallengePeriodLengthUpdate()
  await randomBeaconGovernance
    .connect(governance)
    .finalizeDkgResultSubmissionTimeoutUpdate()
  await randomBeaconGovernance
    .connect(governance)
    .finalizeDkgSubmitterPrecedencePeriodLengthUpdate()

  await randomBeaconGovernance
    .connect(governance)
    .finalizeRelayEntrySubmissionFailureSlashingAmountUpdate()
  await randomBeaconGovernance
    .connect(governance)
    .finalizeMaliciousDkgResultSlashingAmountUpdate()
  await randomBeaconGovernance
    .connect(governance)
    .finalizeUnauthorizedSigningSlashingAmountUpdate()
}

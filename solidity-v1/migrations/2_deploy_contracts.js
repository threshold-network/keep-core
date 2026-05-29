const KeepToken = artifacts.require("./KeepToken.sol")
const ModUtils = artifacts.require("./utils/ModUtils.sol")
const AltBn128 = artifacts.require("./cryptography/AltBn128.sol")
const BLS = artifacts.require("./cryptography/BLS.sol")
const MinimumStakeSchedule = artifacts.require(
  "./libraries/staking/MinimumStakeSchedule.sol"
)
const GrantStaking = artifacts.require("./libraries/staking/GrantStaking.sol")
const Locks = artifacts.require("./libraries/staking/Locks.sol")
const TopUps = artifacts.require("./libraries/staking/TopUps.sol")
const TokenStaking = artifacts.require("./TokenStaking.sol")
const PermissiveStakingPolicy = artifacts.require(
  "./PermissiveStakingPolicy.sol"
)
const GuaranteedMinimumStakingPolicy = artifacts.require(
  "./GuaranteedMinimumStakingPolicy.sol"
)
const TokenGrant = artifacts.require("./TokenGrant.sol")
const ManagedGrantFactory = artifacts.require("./ManagedGrantFactory.sol")
const KeepRandomBeaconService = artifacts.require(
  "./KeepRandomBeaconService.sol"
)
const KeepRandomBeaconServiceImplV1 = artifacts.require(
  "./KeepRandomBeaconServiceImplV1.sol"
)
const KeepRandomBeaconOperator = artifacts.require(
  "./KeepRandomBeaconOperator.sol"
)
const KeepRandomBeaconOperatorStatistics = artifacts.require(
  "./statistics/KeepRandomBeaconOperatorStatistics.sol"
)
const GroupSelection = artifacts.require(
  "./libraries/operator/GroupSelection.sol"
)
const Groups = artifacts.require("./libraries/operator/Groups.sol")
const DKGResultVerification = artifacts.require(
  "./libraries/operator/DKGResultVerification.sol"
)
const Reimbursements = artifacts.require(
  "./libraries/operator/Reimbursements.sol"
)
const DelayFactor = artifacts.require("./libraries/operator/DelayFactor.sol")
const KeepRegistry = artifacts.require("./KeepRegistry.sol")
const GasPriceOracle = artifacts.require("./GasPriceOracle.sol")
const StakingPortBacker = artifacts.require("./StakingPortBacker.sol")
const BeaconRewards = artifacts.require("./BeaconRewards.sol")
const KeepVault = artifacts.require("./geyser/KeepVault.sol")

let initializationPeriod = 43200 // ~12 hours
const dkgContributionMargin = 1 // 1%
const testNetworks = [
  "local",
  "ropsten",
  "keep_dev",
  "alfajores",
  "goerli",
  "sepolia",
]

module.exports = async function (deployer, network) {
  // Set the stake initialization period to 1 block for local development and testnet.
  if (testNetworks.includes(network)) {
    initializationPeriod = 1
  }

  await deployer.deploy(ModUtils)
  await deployer.link(ModUtils, AltBn128)
  await deployer.deploy(AltBn128)
  await deployer.link(AltBn128, BLS)
  await deployer.deploy(BLS)
  await deployer.deploy(KeepToken)
  await deployer.deploy(TokenGrant, KeepToken.address)
  await deployer.deploy(KeepRegistry)
  await deployer.deploy(MinimumStakeSchedule)
  await deployer.deploy(GrantStaking)
  await deployer.deploy(Locks)
  await deployer.deploy(TopUps)
  await deployer.link(MinimumStakeSchedule, TokenStaking)
  await deployer.link(GrantStaking, TokenStaking)
  await deployer.link(Locks, TokenStaking)
  await deployer.link(TopUps, TokenStaking)
  await deployer.deploy(
    TokenStaking,
    KeepToken.address,
    TokenGrant.address,
    KeepRegistry.address,
    initializationPeriod
  )

  let oldStakingContractAddress
  if (testNetworks.includes(network)) {
    const OldTokenStaking = artifacts.require("./stubs/OldTokenStaking.sol")
    await deployer.link(MinimumStakeSchedule, OldTokenStaking)
    await deployer.link(GrantStaking, OldTokenStaking)
    await deployer.link(Locks, OldTokenStaking)
    await deployer.link(TopUps, OldTokenStaking)
    await deployer.deploy(OldTokenStaking, KeepToken.address)
    oldStakingContractAddress = OldTokenStaking.address

    console.log(
      `Deploying StakingPortBacker using old TokenStaking[${oldStakingContractAddress}]`
    )
    await deployer.deploy(
      StakingPortBacker,
      KeepToken.address,
      TokenGrant.address,
      oldStakingContractAddress,
      TokenStaking.address
    )
  }

  await deployer.deploy(PermissiveStakingPolicy)
  await deployer.deploy(GuaranteedMinimumStakingPolicy, TokenStaking.address)
  await deployer.deploy(
    ManagedGrantFactory,
    KeepToken.address,
    TokenGrant.address
  )
  await deployer.deploy(GasPriceOracle)
  await deployer.deploy(GroupSelection)
  await deployer.link(GroupSelection, KeepRandomBeaconOperator)
  await deployer.link(BLS, Groups)
  await deployer.deploy(Groups)
  await deployer.link(Groups, KeepRandomBeaconOperator)
  await deployer.deploy(DKGResultVerification)
  await deployer.link(DKGResultVerification, KeepRandomBeaconOperator)
  await deployer.deploy(DelayFactor)
  await deployer.link(DelayFactor, KeepRandomBeaconOperator)
  await deployer.deploy(Reimbursements)
  await deployer.link(Reimbursements, KeepRandomBeaconOperator)
  await deployer.link(BLS, KeepRandomBeaconOperator)

  const keepRandomBeaconServiceImplV1 = await deployer.deploy(
    KeepRandomBeaconServiceImplV1
  )

  const initialize = keepRandomBeaconServiceImplV1.contract.methods
    .initialize(dkgContributionMargin, KeepRegistry.address)
    .encodeABI()

  await deployer.deploy(
    KeepRandomBeaconService,
    KeepRandomBeaconServiceImplV1.address,
    initialize
  )

  await deployer.deploy(
    KeepRandomBeaconOperator,
    KeepRandomBeaconService.address,
    TokenStaking.address,
    KeepRegistry.address,
    GasPriceOracle.address
  )

  await deployer.deploy(
    KeepRandomBeaconOperatorStatistics,
    KeepRandomBeaconOperator.address
  )

  await deployer.deploy(
    BeaconRewards,
    KeepToken.address,
    KeepRandomBeaconOperator.address,
    TokenStaking.address
  )

  // KEEP token geyser contract
  const maxUnlockSchedules = 12
  const startBonus = 30 // 30%
  const bonusPeriodSec = 2592000 // 30 days in seconds
  const initialSharesPerToken = 1
  const durationSec = 2592000 // 30 days in seconds

  await deployer.deploy(
    KeepVault,
    // KEEP token is a distribution and staking token.
    KeepToken.address,
    maxUnlockSchedules,
    startBonus,
    bonusPeriodSec,
    initialSharesPerToken,
    durationSec
  )
}

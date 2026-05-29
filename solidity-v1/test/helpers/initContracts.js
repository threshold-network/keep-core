const { contract, accounts } = require("@openzeppelin/test-environment")
const { time } = require("@openzeppelin/test-helpers")

const BLS = contract.fromArtifact("BLS")
const GroupSelection = contract.fromArtifact("GroupSelection")
const Groups = contract.fromArtifact("Groups")
const DKGResultVerification = contract.fromArtifact("DKGResultVerification")
const DelayFactor = contract.fromArtifact("DelayFactor")
const Reimbursements = contract.fromArtifact("Reimbursements")
const GasPriceOracle = contract.fromArtifact("GasPriceOracle")
const KeepRegistry = contract.fromArtifact("KeepRegistry")
const KeepToken = contract.fromArtifact("KeepToken")
const TokenGrant = contract.fromArtifact("TokenGrant")
const MinimumStakeSchedule = contract.fromArtifact("MinimumStakeSchedule")
const GrantStaking = contract.fromArtifact("GrantStaking")
const Locks = contract.fromArtifact("Locks")
const TopUps = contract.fromArtifact("TopUps")

async function initTokenStaking(
  tokenAddress,
  tokenGrantAddress,
  keepRegistryAddress,
  stakeInitializationPeriod,
  TokenStaking
) {
  await TokenStaking.detectNetwork()
  await TokenStaking.link(
    "MinimumStakeSchedule",
    (await MinimumStakeSchedule.new({ from: accounts[0] })).address
  )
  await TokenStaking.link(
    "GrantStaking",
    (await GrantStaking.new({ from: accounts[0] })).address
  )
  await TokenStaking.link(
    "Locks",
    (await Locks.new({ from: accounts[0] })).address
  )
  await TokenStaking.link(
    "TopUps",
    (await TopUps.new({ from: accounts[0] })).address
  )

  const tokenStaking = await TokenStaking.new(
    tokenAddress,
    tokenGrantAddress,
    keepRegistryAddress,
    stakeInitializationPeriod,
    { from: accounts[0] }
  )

  return {
    tokenStaking: tokenStaking,
  }
}

async function initContracts(
  TokenStaking,
  KeepRandomBeaconService,
  KeepRandomBeaconServiceImplV1,
  KeepRandomBeaconOperator
) {
  const dkgContributionMargin = 5 // 5% Represents DKG frequency of 1/20 (Every 20 entries trigger group selection)
  const stakeInitializationPeriod = time.duration.hours(6)

  const token = await KeepToken.new({ from: accounts[0] })
  const tokenGrant = await TokenGrant.new(token.address, { from: accounts[0] })
  const registry = await KeepRegistry.new({ from: accounts[0] })

  // Initialize staking contract
  const stakingContracts = await initTokenStaking(
    token.address,
    tokenGrant.address,
    registry.address,
    stakeInitializationPeriod,
    TokenStaking
  )
  const stakingContract = stakingContracts.tokenStaking

  // Initialize Keep Random Beacon service contract
  const serviceContractImplV1 = await KeepRandomBeaconServiceImplV1.new({
    from: accounts[0],
  })

  const initialize = serviceContractImplV1.contract.methods
    .initialize(dkgContributionMargin, registry.address)
    .encodeABI()

  const gasPriceOracle = await GasPriceOracle.new({ from: accounts[0] })

  const serviceContractProxy = await KeepRandomBeaconService.new(
    serviceContractImplV1.address,
    initialize,
    { from: accounts[0] }
  )
  const serviceContract = await KeepRandomBeaconServiceImplV1.at(
    serviceContractProxy.address
  )
  // Initialize Keep Random Beacon operator contract
  const bls = await BLS.new({ from: accounts[0] })
  await KeepRandomBeaconOperator.detectNetwork()
  await KeepRandomBeaconOperator.link("BLS", bls.address)
  const groupSelection = await GroupSelection.new({ from: accounts[0] })
  await Groups.detectNetwork()
  await Groups.link("BLS", bls.address)
  const groups = await Groups.new({ from: accounts[0] })
  const delayFactor = await DelayFactor.new({ from: accounts[0] })
  const dkgResultVerification = await DKGResultVerification.new({
    from: accounts[0],
  })
  const reimbursements = await Reimbursements.new({ from: accounts[0] })

  await KeepRandomBeaconOperator.link("DelayFactor", delayFactor.address)
  await KeepRandomBeaconOperator.link("GroupSelection", groupSelection.address)
  await KeepRandomBeaconOperator.link("Groups", groups.address)
  await KeepRandomBeaconOperator.link(
    "DKGResultVerification",
    dkgResultVerification.address
  )
  await KeepRandomBeaconOperator.link("Reimbursements", reimbursements.address)
  const operatorContract = await KeepRandomBeaconOperator.new(
    serviceContractProxy.address,
    stakingContract.address,
    registry.address,
    gasPriceOracle.address,
    { from: accounts[0] }
  )
  await gasPriceOracle.addConsumerContract(operatorContract.address, {
    from: accounts[0],
  })

  await registry.approveOperatorContract(operatorContract.address, {
    from: accounts[0],
  })

  // Set service contract owner as operator contract upgrader by default
  const operatorContractUpgrader = await serviceContractProxy.admin({
    from: accounts[0],
  })
  await registry.setOperatorContractUpgrader(
    serviceContract.address,
    operatorContractUpgrader,
    { from: accounts[0] }
  )

  await serviceContract.addOperatorContract(operatorContract.address, {
    from: accounts[0],
  })

  const dkgGasEstimate = await operatorContract.dkgGasEstimate({
    from: accounts[0],
  })

  // Genesis should include payment to cover DKG cost to create first group
  const gasPriceCeiling = await operatorContract.gasPriceCeiling({
    from: accounts[0],
  })
  await operatorContract.genesis({
    value: dkgGasEstimate.mul(gasPriceCeiling),
    from: accounts[0],
  })

  return {
    registry: registry,
    token: token,
    stakingContract: stakingContract,
    serviceContract: serviceContract,
    operatorContract: operatorContract,
  }
}

module.exports.initTokenStaking = initTokenStaking
module.exports.initContracts = initContracts

const KeepRandomBeaconService = artifacts.require(
  "./KeepRandomBeaconService.sol"
)
const KeepRandomBeaconServiceImplV1 = artifacts.require(
  "./KeepRandomBeaconServiceImplV1.sol"
)
const KeepRandomBeaconOperator = artifacts.require(
  "./KeepRandomBeaconOperator.sol"
)
const KeepRegistry = artifacts.require("./KeepRegistry.sol")
const TokenStaking = artifacts.require("./TokenStaking.sol")
const TokenGrant = artifacts.require("./TokenGrant.sol")
const GasPriceOracle = artifacts.require("./GasPriceOracle.sol")
const OldTokenStaking = artifacts.require("../stubs/OldTokenStaking.sol")

module.exports = async function (deployer, network) {
  const keepRandomBeaconService = await KeepRandomBeaconService.deployed()
  const keepRandomBeaconServiceImplV1 = await KeepRandomBeaconServiceImplV1.at(
    keepRandomBeaconService.address
  )
  const keepRandomBeaconOperator = await KeepRandomBeaconOperator.deployed()
  const keepRegistry = await KeepRegistry.deployed()
  const tokenStaking = await TokenStaking.deployed()
  const oldTokenStaking = await OldTokenStaking.deployed()
  const tokenGrant = await TokenGrant.deployed()
  const gasPriceOracle = await GasPriceOracle.deployed()

  if (!(await keepRandomBeaconServiceImplV1.initialized())) {
    throw Error("keep random beacon service not initialized")
  }

  await tokenGrant.authorizeStakingContract(tokenStaking.address)
  if (network === "local") {
    await tokenGrant.authorizeStakingContract(oldTokenStaking.address)
  }

  await keepRegistry.approveOperatorContract(keepRandomBeaconOperator.address)

  // Set service contract owner as operator contract upgrader by default
  const operatorContractUpgrader = await keepRandomBeaconService.admin()
  await keepRegistry.setOperatorContractUpgrader(
    keepRandomBeaconServiceImplV1.address,
    operatorContractUpgrader
  )
  keepRandomBeaconServiceImplV1.addOperatorContract(
    keepRandomBeaconOperator.address,
    { from: operatorContractUpgrader }
  )

  await gasPriceOracle.addConsumerContract(keepRandomBeaconOperator.address)
}

// The sibling Threshold package's deployment scripts have their own runtime
// migration. Use its real exported contracts, without executing those scripts.
const func = async ({ deployments, getNamedAccounts }) => {
  const { deployer } = await getNamedAccounts()
  const threshold = "@threshold-network/solidity-contracts/export/artifacts/"
  const token = await deployments.deploy("T", {
    from: deployer,
    contract: require(`${threshold}contracts/token/T.sol/T.json`),
  })
  if (token.newlyDeployed) {
    await deployments.execute(
      "T",
      { from: deployer },
      "mint",
      deployer,
      "1000000000000000000000000"
    )
  }
  const staking = await deployments.deploy("TokenStaking", {
    from: deployer,
    contract: require(`${threshold}contracts/staking/TokenStaking.sol/TokenStaking.json`),
    args: [token.address],
  })
  if (staking.newlyDeployed) {
    await deployments.execute("TokenStaking", { from: deployer }, "initialize")
  }
  return true
}
module.exports = func
func.tags = ["ConsumerDependencies", "T", "TokenStaking"]
func.id = "consumer_dependencies"

module.exports = async ({ deployments, getNamedAccounts }) => {
  const { deployer } = await getNamedAccounts();
  const probe = await deployments.get("Probe");
  await deployments.deploy("DependentProbe", {
    from: deployer, args: [probe.address], log: false,
  });
};
module.exports.tags = ["DependentProbe"];
module.exports.dependencies = ["Probe"];

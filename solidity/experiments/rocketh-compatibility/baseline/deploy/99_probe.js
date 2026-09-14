module.exports = async ({ deployments, getNamedAccounts }) => {
  const { deployer, owner } = await getNamedAccounts();
  await deployments.deploy("Probe", {
    from: deployer, args: [owner, 7], log: false,
    linkedData: { purpose: "compatibility", revision: 1 },
  });
};
module.exports.tags = ["Probe"];

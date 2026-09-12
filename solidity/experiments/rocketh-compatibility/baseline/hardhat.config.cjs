require("hardhat-deploy");

module.exports = {
  solidity: "0.8.17",
  paths: {
    artifacts: "../.run/v1-artifacts",
    cache: "../.run/v1-cache",
    deployments: "../.run/v1-deployments",
  },
  namedAccounts: { deployer: 2, owner: 3 },
  networks: { hardhat: { chainId: 31337, hardfork: "cancun" } },
};

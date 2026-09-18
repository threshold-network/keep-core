const path = require("path")

const ethersMajor = require("ethers").version.split(".")[0]
require(ethersMajor === "6"
  ? "@nomicfoundation/hardhat-ethers"
  : "@nomiclabs/hardhat-ethers")
require("hardhat-deploy")
require("@keep-network/hardhat-helpers")

module.exports = {
  solidity: {
    version: "0.8.17",
    settings: { optimizer: { enabled: true, runs: 200 } },
  },
  networks: {
    hardhat: {
      tags: ["etherscan", "useRandomBeaconChaosnet"],
      mining: { auto: true, interval: 100 },
    },
  },
  namedAccounts: {
    deployer: 1,
    governance: 2,
    chaosnetOwner: 3,
    esdm: 4,
  },
  external: {
    contracts: ["random-beacon", "ecdsa"].map((name) => ({
      artifacts: path.join(
        __dirname,
        "node_modules/@keep-network",
        name,
        "export/artifacts"
      ),
      deploy: path.join(
        __dirname,
        "node_modules/@keep-network",
        name,
        "export/deploy"
      ),
    })),
  },
}

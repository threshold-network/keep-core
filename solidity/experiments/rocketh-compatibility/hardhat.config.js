import { createRequire } from "node:module";
import hardhatEthers from "@nomicfoundation/hardhat-ethers";
import hardhatMocha from "@nomicfoundation/hardhat-mocha";
import networkHelpers from "@nomicfoundation/hardhat-network-helpers";
import hardhatDeploy from "hardhat-deploy";
import openzeppelinUpgrades from "@openzeppelin/hardhat-upgrades";

const require = createRequire(import.meta.url);

export default {
  plugins: [hardhatEthers, hardhatMocha, networkHelpers, hardhatDeploy, openzeppelinUpgrades],
  solidity: {
    version: "0.8.17",
    path: require.resolve("solc/soljson.js"),
    preferWasm: true,
    npmFilesToBuild: [
      "@openzeppelin/contracts-v4/proxy/transparent/TransparentUpgradeableProxy.sol",
      "@openzeppelin/contracts-v4/proxy/transparent/ProxyAdmin.sol",
    ],
    settings: { optimizer: { enabled: true, runs: 200 }, evmVersion: "london" },
  },
  networks: {
    hardhat: {
      type: "edr-simulated",
      chainType: "l1",
      chainId: 31337,
      hardfork: "cancun",
    },
  },
};

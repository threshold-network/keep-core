import { HardhatUserConfig } from "hardhat/config";
import "@nomicfoundation/hardhat-toolbox";
import "hardhat-deploy";

const config: HardhatUserConfig = {
  solidity: {
    version: "0.8.17",
    settings: {
      optimizer: {
        enabled: true,
        runs: 200,
      },
    },
  },
  networks: {
    development: {
      url: "http://localhost:8545",
      chainId: 1101,
      accounts: process.env.DEV_ACCOUNTS_PRIVATE_KEYS
        ? process.env.DEV_ACCOUNTS_PRIVATE_KEYS.split(",")
        : undefined,
    },
  },
  namedAccounts: {
    deployer: {
      default: 0,
    },
  },
};

export default config;


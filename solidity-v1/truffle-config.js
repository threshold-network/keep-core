require("babel-register");
require("babel-polyfill");
const HDWalletProvider = require("@truffle/hdwallet-provider");
const Kit = require("@celo/contractkit");

module.exports = {
  networks: {
    local: {
      host: "localhost",
      port: 8545,
      network_id: "*",
    },
    keep_dev: {
      provider: function () {
        return new HDWalletProvider({
          privateKeys: [process.env.CONTRACT_OWNER_ACCOUNT_PRIVATE_KEY],
          providerOrUrl: "http://localhost:8545",
        });
      },
      gas: 6721975,
      network_id: 1101,
    },

    keep_dev_vpn: {
      provider: function () {
        return new HDWalletProvider({
          privateKeys: [process.env.CONTRACT_OWNER_ACCOUNT_PRIVATE_KEY],
          providerOrUrl: "http://eth-tx-node.default.svc.cluster.local:8545",
        });
      },
      gas: 6721975,
      network_id: 1101,
    },

    sepolia: {
      provider: function () {
        return new HDWalletProvider({
          privateKeys: [process.env.CONTRACT_OWNER_ACCOUNT_PRIVATE_KEY],
          providerOrUrl: process.env.CHAIN_API_URL,
        });
      },
      gas: 6000000,
      network_id: 11155111,
      skipDryRun: true,
      networkCheckTimeout: 120000,
      timeoutBlocks: 200, // # of blocks before a deployment times out  (minimum/default: 50)
    },

    alfajores: {
      provider: function () {
        const kit = Kit.newKit(process.env.CHAIN_API_URL);
        kit.addAccount(process.env.CONTRACT_OWNER_ACCOUNT_PRIVATE_KEY);
        return kit.web3.currentProvider;
      },
      network_id: 44787,
    },
  },

  mocha: {
    useColors: true,
    reporter: "eth-gas-reporter",
    reporterOptions: {
      currency: "USD",
      gasPrice: 21,
      showTimeSpent: true,
    },
  },

  compilers: {
    solc: {
      version: "0.8.20",
      optimizer: {
        enabled: true,
        runs: 200,
      },
    },
  },

  plugins: ["truffle-plugin-verify"],

  api_keys: {
    etherscan: process.env.ETHERSCAN_API_KEY,
  },
};

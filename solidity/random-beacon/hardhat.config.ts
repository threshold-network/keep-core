import "@nomiclabs/hardhat-etherscan"
import "@keep-network/hardhat-local-networks-config"
import "@keep-network/hardhat-helpers"
import "@nomiclabs/hardhat-ethers"
import "hardhat-deploy"
import "@tenderly/hardhat-tenderly"
import "@nomiclabs/hardhat-waffle"
import "hardhat-gas-reporter"
import "hardhat-contract-sizer"
import "@typechain/hardhat"
import "hardhat-dependency-compiler"
import "./tasks"
import "solidity-docgen"

import { task } from "hardhat/config"

import type { HardhatUserConfig } from "hardhat/config"

const thresholdSolidityCompilerConfig = {
  version: "0.8.9",
  settings: {
    optimizer: {
      enabled: true,
      runs: 10,
    },
  },
}

// Configuration for testing environment.
export const testConfig = {
  // How many accounts we expect to define for non-staking related signers, e.g.
  // deployer, thirdParty, governance.
  // It is used as an offset for getting accounts for operators and stakes registration.
  nonStakingAccountsCount: 10,

  // How many roles do we need to define for staking, i.e. stakeOwner, stakingProvider,
  // operator, beneficiary, authorizer.
  stakingRolesCount: 5,

  // Number of operators to register. Should be at least the same as group size.
  operatorsCount: 64,
}

// Optional gas price override (in gwei) used to avoid REPLACEMENT_UNDERPRICED
// when retrying a failed deploy. Validated up-front so a malformed value fails
// fast instead of silently producing a NaN gasPrice.
const gasPriceGwei = process.env.GAS_PRICE_GWEI
  ? Number(process.env.GAS_PRICE_GWEI)
  : undefined
if (
  gasPriceGwei !== undefined &&
  (!Number.isInteger(gasPriceGwei) || gasPriceGwei <= 0)
) {
  throw new Error(
    `Invalid GAS_PRICE_GWEI "${process.env.GAS_PRICE_GWEI}": expected a positive integer (gwei)`
  )
}

const config: HardhatUserConfig = {
  solidity: {
    compilers: [
      {
        version: "0.8.17",
        settings: {
          optimizer: {
            enabled: true,
          },
        },
      },
      {
        version: "0.8.9",
        settings: {
          optimizer: {
            enabled: true,
          },
        },
      },
    ],
    overrides: {
      "@threshold-network/solidity-contracts/contracts/staking/TokenStaking.sol":
        thresholdSolidityCompilerConfig,
    },
  },
  paths: {
    artifacts: "./build",
  },
  networks: {
    hardhat: {
      // Configuring a `MockContract` is a transaction, and a transaction mines
      // a block. Without this, Hardhat forces every block to be at least one
      // second after its parent, so setting up a mock silently advances the
      // chain clock and any test asserting on a deadline boundary inverts.
      // `test/helpers/mock.ts` pins the next block's timestamp before each
      // write; this option is what lets it reuse the current one.
      allowBlocksWithSameTimestamp: true,
      forking: {
        // forking is enabled only if FORKING_URL env is provided
        enabled: !!process.env.FORKING_URL,
        // URL should point to a node with archival data (Alchemy recommended)
        url: process.env.FORKING_URL || "",
        // latest block is taken if FORKING_BLOCK env is not provided
        blockNumber: process.env.FORKING_BLOCK
          ? parseInt(process.env.FORKING_BLOCK, 10)
          : undefined,
      },
      accounts: {
        // Number of accounts that should be predefined on the testing environment.
        count:
          testConfig.nonStakingAccountsCount +
          testConfig.stakingRolesCount * testConfig.operatorsCount,
      },
      // we use higher gas price for tests to obtain more realistic results
      // for gas refund tests than when the default hardhat ~1 gwei gas price is
      // used
      gasPrice: 200000000000, // 200 gwei
      // Ignore contract size on deployment to hardhat network, to be able to
      // deploy stub contracts in tests.
      allowUnlimitedContractSize: process.env.TEST_USE_STUBS_BEACON === "true",
      tags: ["allowStubs"],
    },
    development: {
      url: "http://localhost:8545",
      chainId: 1101,
      tags: ["allowStubs"],
    },
    sepolia: {
      url: process.env.CHAIN_API_URL || "",
      chainId: 11155111,
      accounts: process.env.ACCOUNTS_PRIVATE_KEYS
        ? process.env.ACCOUNTS_PRIVATE_KEYS.split(",")
        : undefined,
      tags: ["etherscan", "tenderly"],
      // Override gas to avoid REPLACEMENT_UNDERPRICED when retrying after a failed deploy
      ...(gasPriceGwei !== undefined && {
        gasPrice: gasPriceGwei * 1e9,
      }),
    },
    mainnet: {
      url: process.env.CHAIN_API_URL || "",
      chainId: 1,
      accounts: process.env.CONTRACT_OWNER_ACCOUNT_PRIVATE_KEY
        ? [process.env.CONTRACT_OWNER_ACCOUNT_PRIVATE_KEY]
        : undefined,
      tags: ["etherscan", "tenderly"],
    },
  },
  // // Define local networks configuration file path to load networks from the file.
  // localNetworksConfig: "./.hardhat/networks.ts",
  tenderly: {
    username: "thesis",
    project: "",
  },
  etherscan: {
    apiKey: process.env.ETHERSCAN_API_KEY,
  },
  namedAccounts: {
    deployer: {
      default: 1,
      sepolia: 0,
      mainnet: 0, // "0x123694886DBf5Ac94DDA07135349534536D14cAf"
    },
    governance: {
      default: 2,
      sepolia: 0,
      mainnet: "0x9f6e831c8f8939dc0c830c6e492e7cef4f9c2f5f", // Threshold Council
    },
    chaosnetOwner: {
      default: 3,
      sepolia: 0,
      mainnet: "0x9f6e831c8f8939dc0c830c6e492e7cef4f9c2f5f", // Threshold Council
    },
  },
  external: {
    contracts:
      process.env.USE_EXTERNAL_DEPLOY === "true"
        ? [
            {
              artifacts:
                "node_modules/@threshold-network/solidity-contracts/export/artifacts",
              deploy:
                "node_modules/@threshold-network/solidity-contracts/export/deploy",
            },
          ]
        : undefined,
    deployments: {
      // For hardhat environment we can fork the mainnet, so we need to point it
      // to the contract artifacts.
      hardhat: process.env.FORKING_URL ? ["./external/mainnet"] : [],
      // For development environment we expect the local dependencies to be linked
      // with `yarn link` command.
      development: [
        "node_modules/@threshold-network/solidity-contracts/deployments/development",
      ],
      // Use local deployments/sepolia only - npm artifacts have transactionHash
      // that causes "cannot get the transaction" errors with some RPC nodes.
      sepolia: [],
      mainnet: ["./external/mainnet"],
    },
  },
  dependencyCompiler: {
    paths: [
      "@threshold-network/solidity-contracts/contracts/token/T.sol",
      "@threshold-network/solidity-contracts/contracts/staking/TokenStaking.sol",
    ],
    keep: true,
  },
  contractSizer: {
    alphaSort: true,
    disambiguatePaths: false,
    runOnCompile: true,
    strict: true,
    except: ["^contracts/test"],
  },
  mocha: {
    timeout: 60000,
  },
  typechain: {
    outDir: "typechain",
  },
  gasReporter: {
    currency: "USD",
    coinmarketcap: process.env.COINMARKETCAP_API_KEY,
  },
  docgen: {
    outputDir: "generated-docs",
    templates: "docgen-templates",
    pages: "files", // `single`, `items` or `files`
    exclude: ["./test"],
  },
}

task("check-accounts-count", "Checks accounts count").setAction(async () => {
  // eslint-disable-next-line @typescript-eslint/no-var-requires,global-require
  const { constants } = require("./test/fixtures")

  if (testConfig.operatorsCount < constants.groupSize) {
    throw new Error(
      "not enough accounts predefined for configured group size: " +
        `expected group size: ${constants.groupSize} ` +
        `number of predefined accounts: ${testConfig.operatorsCount}`
    )
  }
})

export default config

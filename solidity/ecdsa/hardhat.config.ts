import "@nomicfoundation/hardhat-verify"
import "@keep-network/hardhat-helpers"
import "@keep-network/hardhat-local-networks-config"
import "@nomiclabs/hardhat-waffle"
import "@nomicfoundation/hardhat-chai-matchers"
import "@openzeppelin/hardhat-upgrades"
import "@typechain/hardhat"
import "hardhat-deploy"
import "@tenderly/hardhat-tenderly"
import "hardhat-contract-sizer"
import "hardhat-dependency-compiler"
import "hardhat-gas-reporter"
import "solidity-docgen"

import "./tasks"
import { task } from "hardhat/config"
import { TASK_TEST } from "hardhat/builtin-tasks/task-names"

import type { HardhatUserConfig } from "hardhat/config"

const TASK_CHECK_ACCOUNTS_COUNT = "check-accounts-count"

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
  operatorsCount: 100,
}

const config: HardhatUserConfig = {
  solidity: {
    compilers: [
      {
        version: "0.8.17",
        settings: {
          optimizer: {
            enabled: true,
            runs: 200,
          },
        },
      },
      {
        version: "0.8.9",
        settings: {
          optimizer: {
            enabled: true,
            runs: 200,
          },
        },
      },
    ],
    overrides: {
      "@threshold-network/solidity-contracts/contracts/staking/TokenStaking.sol":
        thresholdSolidityCompilerConfig,
    },
    settings: {
      outputSelection: {
        "*": {
          "*": ["storageLayout"],
        },
      },
    },
  },
  paths: {
    artifacts: "./build",
  },
  networks: {
    hardhat: {
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
      allowUnlimitedContractSize: process.env.TEST_USE_STUBS_ECDSA === "true",
      tags: ["allowStubs"],
    },
    development: {
      url: "http://localhost:8545",
      chainId: 1101,
      tags: ["allowStubs", "useRandomBeaconChaosnet"],
    },
    sepolia: {
      url: process.env.CHAIN_API_URL || "",
      chainId: 11155111,
      accounts: process.env.ACCOUNTS_PRIVATE_KEYS
        ? process.env.ACCOUNTS_PRIVATE_KEYS.split(",")
        : undefined,
      tags: ["etherscan", "tenderly", "useRandomBeaconChaosnet"],
    },
    mainnet: {
      url: process.env.CHAIN_API_URL || "",
      chainId: 1,
      accounts: process.env.CONTRACT_OWNER_ACCOUNT_PRIVATE_KEY
        ? [process.env.CONTRACT_OWNER_ACCOUNT_PRIVATE_KEY]
        : undefined,
      tags: ["etherscan", "tenderly", "useRandomBeaconChaosnet"],
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
      default: 1, // take the second account
      sepolia: "0x68ad60CC5e8f3B7cC53beaB321cf0e6036962dBc",
      mainnet: "0x716089154304f22a2F9c8d2f8C45815183BF3532",
    },
    governance: {
      default: 2,
      sepolia: "0x68ad60CC5e8f3B7cC53beaB321cf0e6036962dBc",
      mainnet: "0x9f6e831c8f8939dc0c830c6e492e7cef4f9c2f5f", // Threshold Council
    },
    chaosnetOwner: {
      default: 3,
      sepolia: "0x68ad60CC5e8f3B7cC53beaB321cf0e6036962dBc",
      mainnet: "0x9f6e831c8f8939dc0c830c6e492e7cef4f9c2f5f", // Threshold Council
    },
    esdm: {
      default: 4,
      sepolia: "0x68ad60CC5e8f3B7cC53beaB321cf0e6036962dBc",
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
            {
              artifacts:
                "node_modules/@keep-network/random-beacon/export/artifacts",
              deploy: "node_modules/@keep-network/random-beacon/export/deploy",
            },
          ]
        : undefined,
    deployments: {
      // For hardhat environment we can fork the mainnet, so we need to point it
      // to the contract artifacts.
      // hardhat: process.env.FORKING_URL ? ["./external/mainnet"] : [],
      // For development environment we expect the local dependencies to be linked
      // with `yarn link` command.
      development: [
        "node_modules/@threshold-network/solidity-contracts/deployments/development",
        "node_modules/@keep-network/random-beacon/deployments/development",
      ],
      sepolia: [
        "./external/sepolia",
        "node_modules/@threshold-network/solidity-contracts/artifacts",
        "node_modules/@keep-network/random-beacon/artifacts",
      ],
      mainnet: ["./external/mainnet"],
    },
  },
  dependencyCompiler:
    // As a workaround for a slither issue https://github.com/crytic/slither/issues/1140
    // we disable compilation of dependencies when running slither.
    process.env.SKIP_DEPENDENCY_COMPILER === "true"
      ? undefined
      : {
          paths: [
            "@threshold-network/solidity-contracts/contracts/token/T.sol",
            "@threshold-network/solidity-contracts/contracts/staking/TokenStaking.sol",
            "@keep-network/random-beacon/contracts/api/IRandomBeacon.sol",
            "@openzeppelin/contracts/proxy/transparent/TransparentUpgradeableProxy.sol",
          ],
          keep: true,
        },
  contractSizer: {
    alphaSort: true,
    disambiguatePaths: false,
    runOnCompile: true,
    strict: true,
    except: ["contracts/test"],
  },
  mocha: {
    timeout: 60000,
  },
  typechain: {
    outDir: "typechain",
  },
  docgen: {
    outputDir: "generated-docs",
    templates: "docgen-templates",
    pages: "files", // `single`, `items` or `files`
    exclude: ["./test"],
  },
}

task(TASK_TEST, "Runs mocha tests").setAction(async (args, hre, runSuper) => {
  await hre.run(TASK_CHECK_ACCOUNTS_COUNT)

  return runSuper(args)
})

task(TASK_CHECK_ACCOUNTS_COUNT, "Checks accounts count").setAction(async () => {
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

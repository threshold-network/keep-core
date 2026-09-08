const fs = require("node:fs");
const path = require("node:path");
const { createRequire } = require("node:module");

const requirePackage = createRequire(path.join(process.cwd(), "package.json"));
requirePackage("ts-node/register/transpile-only");
const { extendProvider, task } = requirePackage("hardhat/config");
const { ProviderWrapper } = requirePackage("hardhat/plugins");
// Loading the packed configuration also registers only its own task exports.
// Registering source and packed tasks together would redefine required params.
const ecdsaExport = process.env.ECDSA_EXPORT_PATH;
const packageConfig = require(
  ecdsaExport
    ? path.resolve(ecdsaExport, "hardhat.config.js")
    : path.join(process.cwd(), "hardhat.config.ts"),
);
const config = packageConfig.default;

class FixedClockProvider extends ProviderWrapper {
  async request(args) {
    if (
      ["eth_sendTransaction", "eth_sendRawTransaction"].includes(args.method)
    ) {
      const latest = await this._wrapped.request({
        method: "eth_getBlockByNumber",
        params: ["latest", false],
      });
      await this._wrapped.request({
        method: "evm_setNextBlockTimestamp",
        params: [Number.parseInt(latest.timestamp, 16) + 1],
      });
    }
    return this._wrapped.request(args);
  }
}

extendProvider(async (provider, resolvedConfig, network) => {
  if (
    network !== "hardhat" ||
    resolvedConfig.networks.hardhat.forking?.enabled
  ) {
    throw new Error(
      "Compatibility capture requires non-forked in-process Hardhat",
    );
  }
  return new FixedClockProvider(provider);
});

// Test the executable ECDSA export from an actual packed producer.
if (ecdsaExport) {
  // The full-suite account guard imports test fixtures omitted from npm.
  // These task tests need only the named slots and one set of staking roles.
  task("check-accounts-count").setAction(async (_args, hre) => {
    const { nonStakingAccountsCount, stakingRolesCount } =
      packageConfig.testConfig;
    const required = nonStakingAccountsCount + stakingRolesCount;
    if ((await hre.ethers.getSigners()).length < required) {
      throw new Error(
        `Packed task checks require at least ${required} accounts`,
      );
    }
  });
  const root = path.resolve(ecdsaExport);
  for (const subdir of ["deploy", "artifacts", "tasks"]) {
    if (!fs.statSync(path.join(root, subdir)).isDirectory()) {
      throw new Error(`ECDSA ${subdir} export is missing: ${root}`);
    }
  }
  config.paths = { ...config.paths, deploy: path.join(root, "deploy") };
  config.external = {
    ...config.external,
    contracts: [
      ...(config.external?.contracts ?? []),
      { artifacts: path.join(root, "artifacts") },
    ],
  };
}

module.exports = {
  ...config,
  paths: { ...config.paths, root: process.cwd() },
  networks: {
    ...config.networks,
    hardhat: {
      ...config.networks.hardhat,
      initialDate: "2024-01-01T00:00:00.000Z",
    },
  },
};

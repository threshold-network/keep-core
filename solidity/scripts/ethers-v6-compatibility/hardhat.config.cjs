const fs = require("node:fs");
const path = require("node:path");
const { createRequire } = require("node:module");

const requirePackage = createRequire(path.join(process.cwd(), "package.json"));
requirePackage("ts-node/register/transpile-only");
const { extendProvider } = requirePackage("hardhat/config");
const { ProviderWrapper } = requirePackage("hardhat/plugins");
const config = require(path.join(process.cwd(), "hardhat.config.ts")).default;

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
const ecdsaExport = process.env.ECDSA_EXPORT_PATH;
if (ecdsaExport) {
  const root = path.resolve(ecdsaExport);
  for (const subdir of ["deploy", "artifacts"]) {
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

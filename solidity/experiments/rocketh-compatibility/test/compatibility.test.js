import { expect } from "chai";
import { readFileSync, writeFileSync, existsSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import hre from "hardhat";
import { upgrades } from "@openzeppelin/hardhat-upgrades";
import { setupEnvironmentFromFiles } from "@rocketh/node";
import { resolveConfig } from "rocketh";
import { run as exportRocketh } from "@rocketh/export";
import { legacyExport } from "../scripts/legacy-export.mjs";
import { extensions } from "@keep-test/rocketh-producer/rocketh";

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
const producerDir = path.dirname(fileURLToPath(import.meta.resolve("@keep-test/rocketh-producer/deploy")));
const { loadAndExecuteDeploymentsFromFilesWithConfig } = setupEnvironmentFromFiles(extensions);
const config = {
  deployments: path.join(root, ".run", "v2-deployments"),
  scripts: [path.join(root, "consumer-deploy"), producerDir],
  accounts: { deployer: { default: 2 }, owner: { default: 3 } },
  environments: { hardhat: { chain: 31337 } },
  chains: { 31337: { info: {
    id: 31337, name: "Hardhat", nativeCurrency: { name: "Ether", symbol: "ETH", decimals: 18 },
    rpcUrls: { default: { http: [] } },
  } } },
  defaultPollingInterval: 10,
};
let connection;
let signers;

async function executeDeployments() {
  return loadAndExecuteDeploymentsFromFilesWithConfig({
    provider: connection.provider,
    environment: "hardhat",
    tags: ["DependentProbe"],
    saveDeployments: true,
    autoMine: true,
    askBeforeProceeding: false,
  }, config);
}

async function deploymentFixture() {
  const env = await executeDeployments();
  const probe = await connection.ethers.getContractAt("Probe", env.get("Probe").address, signers[3]);
  const dependent = await connection.ethers.getContractAt("DependentProbe", env.get("DependentProbe").address);
  return { env, probe, dependent };
}

describe("packed Rocketh deployment API", function () {
  this.timeout(60000);

  before(async () => {
    connection = await hre.network.create({ network: "hardhat" });
    signers = await connection.ethers.getSigners();
  });

  after(async () => { await connection.close(); });

  it("loads the npm tarball and honors cross-package dependencies, tags and consumer accounts", async () => {
    expect(producerDir).to.include("/node_modules/@keep-test/rocketh-producer/");
    expect(existsSync(path.join(root, ".run", "producer"))).to.equal(false);
    const { env, probe, dependent } = await connection.networkHelpers.loadFixture(deploymentFixture);
    expect(await probe.owner()).to.equal(signers[3].address);
    expect(await probe.value()).to.equal(7n);
    expect((await dependent.probe()).toLowerCase()).to.equal((await probe.getAddress()).toLowerCase());
    expect(Object.keys(env.deployments).sort()).to.deep.equal(["DependentProbe", "Probe"]);
    expect(env.namedAccounts.deployer.toLowerCase()).to.equal(signers[2].address.toLowerCase());
  });

  it("reruns deployment scripts without creating another contract or transaction", async () => {
    const { env } = await connection.networkHelpers.loadFixture(deploymentFixture);
    const before = await connection.provider.request({ method: "eth_blockNumber" });
    const rerun = await executeDeployments();
    expect(rerun.get("Probe").address).to.equal(env.get("Probe").address);
    expect(rerun.get("DependentProbe").address).to.equal(env.get("DependentProbe").address);
    expect(await connection.provider.request({ method: "eth_blockNumber" })).to.equal(before);
  });

  it("restores Rocketh deployments through a Hardhat ethers v6 fixture", async () => {
    const { probe } = await connection.networkHelpers.loadFixture(deploymentFixture);
    await (await probe.setValue(44n)).wait();
    expect(await probe.value()).to.equal(44n);
    const restored = await connection.networkHelpers.loadFixture(deploymentFixture);
    expect(await restored.probe.value()).to.equal(7n);
  });

  it("matches the v1 deployment addresses and ABIs from a separate Hardhat 2 run", async () => {
    const { env } = await connection.networkHelpers.loadFixture(deploymentFixture);
    const baseline = JSON.parse(readFileSync(path.join(root, ".run", "v1-export.json"), "utf8"));
    expect(Object.keys(baseline.contracts).sort()).to.deep.equal(["DependentProbe", "Probe"]);
    for (const name of Object.keys(baseline.contracts)) {
      expect(env.get(name).address.toLowerCase()).to.equal(baseline.contracts[name].address.toLowerCase());
      expect(env.get(name).abi).to.deep.equal(baseline.contracts[name].abi);
    }
    writeFileSync(path.join(root, ".run", "v2-records.json"),
      JSON.stringify(env.deployments, (_, value) => typeof value === "bigint" ? value.toString() : value, 2));
  });

  it("reproduces the v1 export JSON byte for byte, including linkedData and checksum addresses", async () => {
    const { env } = await connection.networkHelpers.loadFixture(deploymentFixture);
    const output = legacyExport(env);
    writeFileSync(path.join(root, ".run", "v2-legacy-export.json"), output);
    expect(output).to.equal(readFileSync(path.join(root, ".run", "v1-export.json"), "utf8"));
  });

  it("confirms native Rocketh export has a different schema", async () => {
    await connection.networkHelpers.loadFixture(deploymentFixture);
    const output = path.join(root, ".run", "v2-native-export.json");
    await exportRocketh(await resolveConfig(config), "hardhat", { tojson: [output] });
    const native = JSON.parse(readFileSync(output, "utf8"));
    expect(native).to.have.property("chain");
    expect(native).not.to.have.property("chainId");
    expect(native.contracts.Probe).to.have.property("startBlock");
  });
});

describe("legacy proxy behavior on the Hardhat 3 upgrades API", function () {
  this.timeout(60000);
  let connection;
  let ethers;
  let api;
  let signer;
  let governor;
  let admin;
  let first;
  let second;

  before(async () => {
    connection = await hre.network.create({ network: "hardhat" });
    ethers = connection.ethers;
    [signer, governor] = await ethers.getSigners();
    api = await upgrades(hre, connection);
    const implementationFactory = await ethers.getContractFactory("ProxyProbe");
    const implementation = await api.deployImplementation(implementationFactory, { kind: "transparent" });
    admin = await ethers.deployContract("ProxyAdmin");
    await admin.waitForDeployment();
    const proxyFactory = await ethers.getContractFactory("TransparentUpgradeableProxy");
    async function deployLegacyProxy(initialValue) {
      const init = implementationFactory.interface.encodeFunctionData("initialize", [initialValue]);
      const proxy = await proxyFactory.deploy(implementation, await admin.getAddress(), init);
      await proxy.waitForDeployment();
      return api.forceImport(await proxy.getAddress(), implementationFactory, { kind: "transparent" });
    }
    first = await deployLegacyProxy(7n);
    second = await deployLegacyProxy(8n);
  });

  after(async () => { await connection.close(); });

  it("records and supports two legacy proxies sharing an admin", async () => {
    expect(await api.erc1967.getAdminAddress(await first.getAddress())).to.equal(await admin.getAddress());
    expect(await api.erc1967.getAdminAddress(await second.getAddress())).to.equal(await admin.getAddress());
    expect(await first.value()).to.equal(7n);
    expect(await second.value()).to.equal(8n);
  });

  it("rejects an incompatible storage layout", async () => {
    const badFactory = await ethers.getContractFactory("ProxyProbeBadLayout");
    let rejection;
    try {
      await api.validateUpgrade(await first.getAddress(), badFactory, { kind: "transparent" });
    } catch (error) { rejection = error; }
    expect(rejection, "storage-layout validation must reject the changed value type").to.be.instanceOf(Error);
    expect(rejection.message).to.match(/storage layout|incompatible|type.*changed/i);
  });

  it("preserves storage and the shared admin when upgrading an existing proxy", async () => {
    const v2 = await ethers.getContractFactory("ProxyProbeV2");
    const upgraded = await api.upgradeProxy(await first.getAddress(), v2);
    await upgraded.waitForDeployment();
    expect(await upgraded.value()).to.equal(7n);
    expect(await upgraded.version()).to.equal(2n);
    expect(await api.erc1967.getAdminAddress(await first.getAddress())).to.equal(await admin.getAddress());
    expect(await second.value()).to.equal(8n);
  });

  it("supports ownership transfer and replacing one legacy proxy's admin", async () => {
    await (await admin.transferOwnership(governor.address)).wait();
    expect(await admin.owner()).to.equal(governor.address);
    const replacement = await ethers.deployContract("ProxyAdmin", [], governor);
    await replacement.waitForDeployment();
    await (await admin.connect(governor).changeProxyAdmin(await first.getAddress(), await replacement.getAddress())).wait();
    expect(await api.erc1967.getAdminAddress(await first.getAddress())).to.equal(await replacement.getAddress());
    expect(await api.erc1967.getAdminAddress(await second.getAddress())).to.equal(await admin.getAddress());
  });
});

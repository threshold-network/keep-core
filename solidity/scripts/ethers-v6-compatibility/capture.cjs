const fs = require("fs");
const path = require("path");
require(
  path.join(process.cwd(), "node_modules/ts-node/register/transpile-only"),
);
const hre = require(path.join(process.cwd(), "node_modules/hardhat"));
(async () => {
  if (hre.network.name !== "hardhat" || hre.network.config.forking?.enabled)
    throw new Error("Capture requires a non-forked in-process Hardhat network");
  if (
    process.env.TEST_USE_STUBS_BEACON === "true" ||
    process.env.TEST_USE_STUBS_ECDSA === "true"
  )
    throw new Error(
      "Capture requires production contracts, without test stubs",
    );
  const destination = process.env.MIGRATION_CAPTURE_DIR;
  if (!destination) throw new Error("MIGRATION_CAPTURE_DIR is required");
  if (fs.existsSync(destination))
    throw new Error("Capture destination already exists");
  fs.mkdirSync(destination, { recursive: true });
  fs.writeFileSync(
    path.join(destination, "inputs.json"),
    JSON.stringify(
      {
        deploy: hre.config.paths.deploy,
        externalContracts: hre.config.external?.contracts,
      },
      null,
      2,
    ) + "\n",
  );
  await hre.run("deploy", {
    reset: true,
    write: false,
    export: path.join(destination, "export.json"),
  });
  const records = await hre.deployments.all();
  const send = (method, params) => hre.network.provider.send(method, params);
  const id = hre.ethers.id || hre.ethers.utils.id;
  const slot = (label) =>
    "0x" + (BigInt(id(label)) - 1n).toString(16).padStart(64, "0");
  const adminSlot = slot("eip1967.proxy.admin");
  const implementationSlot = slot("eip1967.proxy.implementation");
  const state = { contracts: {}, code: {} };
  for (const [name, record] of Object.entries(records)) {
    const address = record.address;
    const adminWord = await send("eth_getStorageAt", [
      address,
      adminSlot,
      "latest",
    ]);
    const implementationWord = await send("eth_getStorageAt", [
      address,
      implementationSlot,
      "latest",
    ]);
    const row = { address, adminWord, implementationWord, readers: {} };
    for (const getter of ["owner", "governance", "walletOwner"]) {
      if (
        record.abi.some(
          (entry) =>
            entry.type === "function" &&
            entry.name === getter &&
            entry.inputs.length === 0,
        )
      ) {
        row.readers[getter] = await send("eth_call", [
          { to: address, data: id(getter + "()").slice(0, 10) },
          "latest",
        ]);
      }
    }
    state.contracts[name] = row;
    for (const target of [
      address,
      "0x" + adminWord.slice(-40),
      "0x" + implementationWord.slice(-40),
    ]) {
      if (BigInt(target) === 0n) continue;
      state.code[target.toLowerCase()] = await send("eth_getCode", [
        target,
        "latest",
      ]);
    }
    if (BigInt(adminWord) !== 0n)
      row.adminOwner = await send("eth_call", [
        { to: "0x" + adminWord.slice(-40), data: id("owner()").slice(0, 10) },
        "latest",
      ]);
  }
  const blocks = [];
  const height = Number(BigInt(await send("eth_blockNumber", [])));
  for (let number = 0; number <= height; number++) {
    const block = await send("eth_getBlockByNumber", [
      "0x" + number.toString(16),
      true,
    ]);
    const receipts = [];
    for (const transaction of block.transactions)
      receipts.push(
        await send("eth_getTransactionReceipt", [transaction.hash]),
      );
    blocks.push({ block, receipts });
  }
  const artifactRoot = path.join(process.cwd(), "export/artifacts");
  if (!fs.existsSync(artifactRoot))
    throw new Error("Run prepack before capturing exported artifacts");
  fs.cpSync(artifactRoot, path.join(destination, "artifacts"), {
    recursive: true,
  });
  const tokenStakingName =
    "@threshold-network/solidity-contracts/contracts/staking/TokenStaking.sol:TokenStaking";
  const buildInfo = await hre.artifacts.getBuildInfo(tokenStakingName);
  const compilerLayout =
    buildInfo?.output.contracts[tokenStakingName.split(":")[0]].TokenStaking
      .storageLayout;
  fs.writeFileSync(
    path.join(destination, "compiler-storage-layout.json"),
    JSON.stringify(compilerLayout ?? null, null, 2) + "\n",
  );
  fs.writeFileSync(
    path.join(destination, "chain.json"),
    JSON.stringify(blocks, null, 2) + "\n",
  );
  fs.writeFileSync(
    path.join(destination, "state.json"),
    JSON.stringify(state, null, 2) + "\n",
  );
  fs.writeFileSync(
    path.join(destination, "deployments.json"),
    JSON.stringify(records, null, 2) + "\n",
  );
  console.log(
    `Captured ${Object.keys(records).length} deployments and ${Object.keys(state.code).length} code addresses`,
  );
})().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});

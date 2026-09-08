// Usage: node compare.cjs BASELINE_CAPTURE CANDIDATE_CAPTURE [--ethers-v6]
// The optional flag permits only the reviewed Hardhat 2 ethers v5 -> v6 gas
// changes. Without it, packed-package/fallback captures must match exactly.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { createHash } = require("node:crypto");

const [baseline, candidate, mode] = process.argv.slice(2);
assert(baseline && candidate, "Two capture directories are required");
assert(!mode || mode === "--ethers-v6", "Unknown comparison mode");
const read = (root, file) => JSON.parse(fs.readFileSync(path.join(root, file)));
const byteEqual = (file) =>
  fs
    .readFileSync(path.join(baseline, file))
    .equals(fs.readFileSync(path.join(candidate, file)));
for (const file of ["export.json", "state.json"]) {
  assert(byteEqual(file), `${file} differs`);
}

const before = read(baseline, "chain.json");
const after = read(candidate, "chain.json");
assert.equal(after.length, before.length, "Block count differs");
const isEcdsa = Object.hasOwn(
  read(candidate, "deployments.json"),
  "WalletRegistry",
);
const gasChanges =
  mode && isEcdsa
    ? new Map([
        [33, 5222326],
        [34, 443289],
        [35, 1065225],
        [42, 15799896],
        [45, 809383],
        [46, 683960],
      ])
    : new Map();
const hashes = new Map();
const changedTransactions = new Set();
for (let i = 0; i < before.length; i++) {
  const a = before[i].block;
  const b = after[i].block;
  assert.equal(b.stateRoot, a.stateRoot, `State root differs at block ${i}`);
  assert.equal(b.transactions.length, a.transactions.length);
  if (gasChanges.size && i >= 33) hashes.set(b.hash, a.hash);
  if (gasChanges.has(i)) {
    assert.equal(a.transactions.length, 1);
    const oldTx = a.transactions[0];
    const newTx = b.transactions[0];
    assert.equal(Number(BigInt(oldTx.gas)), gasChanges.get(i));
    assert.equal(Number(BigInt(newTx.gas)), 16777216);
    hashes.set(newTx.hash, oldTx.hash);
    changedTransactions.add(oldTx.hash);
  }
}

// Only hash-bearing fields are translated. Calldata, topics, bytecode, storage,
// ABI fields and all receipt fields other than hashes remain strict comparisons.
function translateHashes(value, key = "") {
  if (Array.isArray(value)) return value.map((entry) => translateHashes(entry));
  if (value && typeof value === "object") {
    return Object.fromEntries(
      Object.entries(value).map(([name, entry]) => [
        name,
        translateHashes(entry, name),
      ]),
    );
  }
  return ["hash", "parentHash", "blockHash", "transactionHash"].includes(key)
    ? (hashes.get(value) ?? value)
    : value;
}

const normalizedBefore = structuredClone(before);
const normalizedAfter = translateHashes(after);
for (const i of gasChanges.keys()) {
  for (const chain of [normalizedBefore, normalizedAfter]) {
    // These two headers encode the differently signed transaction. Everything
    // else, including receiptsRoot, gasUsed, base fee and timestamp, must match.
    delete chain[i].block.size;
    delete chain[i].block.transactionsRoot;
    const transaction = chain[i].block.transactions[0];
    assert(changedTransactions.has(transaction.hash));
    for (const field of ["gas", "r", "s", "v"]) delete transaction[field];
  }
}
assert.deepEqual(normalizedAfter, normalizedBefore, "Chain execution differs");
assert.deepEqual(
  translateHashes(read(candidate, "deployments.json")),
  read(baseline, "deployments.json"),
  "Deployment records differ beyond reviewed transaction/block hashes",
);

function inventory(root, relative = "") {
  return fs
    .readdirSync(path.join(root, relative), { withFileTypes: true })
    .flatMap((entry) => {
      const name = path.join(relative, entry.name);
      return entry.isDirectory() ? inventory(root, name) : [name];
    })
    .sort();
}
const oldArtifacts = path.join(baseline, "artifacts");
const newArtifacts = path.join(candidate, "artifacts");
const files = inventory(oldArtifacts);
assert.deepEqual(inventory(newArtifacts), files, "Artifact inventory differs");
const additions = [];
for (const file of files) {
  const oldBytes = fs.readFileSync(path.join(oldArtifacts, file));
  const newBytes = fs.readFileSync(path.join(newArtifacts, file));
  if (oldBytes.equals(newBytes)) continue;
  assert(mode, `Artifact bytes differ: ${file}`);
  assert.equal(
    file,
    "@threshold-network/solidity-contracts/contracts/staking/TokenStaking.sol/TokenStaking.json",
  );
  const oldArtifact = JSON.parse(oldBytes);
  const newArtifact = JSON.parse(newBytes);
  assert(!Object.hasOwn(oldArtifact, "storageLayout"));
  assert(newArtifact.storageLayout?.storage?.length > 0);
  assert.deepEqual(
    newArtifact.storageLayout,
    read(candidate, "compiler-storage-layout.json"),
    "Layout differs from compiler output",
  );
  delete newArtifact.storageLayout;
  assert.deepEqual(newArtifact, oldArtifact, "Other artifact fields differ");
  additions.push(`${file}: compiler storageLayout added`);
}
console.log(
  JSON.stringify(
    {
      exportSha256: createHash("sha256")
        .update(fs.readFileSync(path.join(candidate, "export.json")))
        .digest("hex"),
      contracts: Object.keys(read(candidate, "deployments.json")).length,
      blocksWithIdenticalState: before.length - 1,
      deploymentRecordsByteIdentical: byteEqual("deployments.json"),
      reviewedGasLimitChanges: [...gasChanges.keys()],
      artifacts: files.length,
      artifactAdditions: additions,
    },
    null,
    2,
  ),
);

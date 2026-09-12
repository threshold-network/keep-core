// Run from solidity/ecdsa; packing runs each package's prepack step:
// node ../scripts/ethers-v6-compatibility/pack-and-capture.cjs NEW_DIRECTORY REFERENCE_CAPTURE
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { execFileSync } = require("node:child_process");

const [destination, reference] = process.argv.slice(2);
assert(
  destination && reference,
  "A new destination and reference capture are required",
);
assert.equal(
  JSON.parse(fs.readFileSync("package.json")).name,
  "@keep-network/ecdsa",
);
const root = path.resolve(destination);
assert(!fs.existsSync(root), "Destination already exists");
fs.mkdirSync(root, { recursive: true });
const consumerModules = path.join(root, "node_modules");
fs.mkdirSync(path.join(consumerModules, "@keep-network"), { recursive: true });

// Reuse the installed consumer runtime/plugins. Producer contents come solely
// from the archives; no source-directory or sibling-export symlinks are used.
const installed = path.join(process.cwd(), "node_modules");
for (const name of fs.readdirSync(installed)) {
  if (name.startsWith(".")) continue;
  if (name === "@keep-network") {
    for (const dependency of fs.readdirSync(path.join(installed, name))) {
      if (["random-beacon", "ecdsa"].includes(dependency)) continue;
      fs.symlinkSync(
        path.join(installed, name, dependency),
        path.join(consumerModules, name, dependency),
        "dir",
      );
    }
  } else {
    fs.symlinkSync(
      path.join(installed, name),
      path.join(consumerModules, name),
      "dir",
    );
  }
}

const packages = {};
for (const name of ["random-beacon", "ecdsa"]) {
  const source = path.resolve("..", name);
  // Lifecycle scripts stay enabled so prepack regenerates export/ for the
  // archive. npm runs prepare/prepack for `npm pack`, never prepublishOnly.
  const output = execFileSync(
    "npm",
    ["pack", "--json", "--pack-destination", root],
    { cwd: source, encoding: "utf8" },
  );
  const [packed] = JSON.parse(output);
  const target = path.join(consumerModules, "@keep-network", name);
  fs.mkdirSync(target);
  execFileSync("tar", [
    "-xzf",
    path.join(root, packed.filename),
    "--strip-components=1",
    "-C",
    target,
  ]);
  for (const file of packed.files) {
    if (
      !file.path.startsWith("export/") &&
      !file.path.startsWith("external/random-beacon-export/")
    )
      continue;
    assert(
      fs
        .readFileSync(path.join(source, file.path))
        .equals(fs.readFileSync(path.join(target, file.path))),
      `Packed file differs: ${name}/${file.path}`,
    );
  }
  const artifactCount = packed.files.filter((file) =>
    file.path.startsWith("export/artifacts/"),
  ).length;
  const expectedArtifacts = name === "random-beacon" ? 51 : 52;
  assert.equal(
    artifactCount,
    expectedArtifacts,
    `${name} packed ${artifactCount} export/artifacts entries, ` +
      `expected ${expectedArtifacts}. Update it here and the artifact ` +
      `inventory row in solidity/docs/ethers-v6-compatibility.md`,
  );
  if (name === "random-beacon") {
    assert(
      packed.files.some(
        (file) => file.path === "export/utils/wait-for-confirmations.js",
      ),
    );
  } else {
    for (const required of [
      "export/tasks/random-beacon.js",
      "export/utils/random-beacon-export.js",
      "external/random-beacon-export/tasks/initialize.js",
      "external/random-beacon-export/tasks/unlock-eth-accounts.js",
      "external/random-beacon-export/tasks/utils/index.js",
    ]) {
      assert(
        packed.files.some((file) => file.path === required),
        required,
      );
    }
    const prefix = "external/random-beacon-export/";
    for (const file of packed.files) {
      if (!file.path.startsWith(prefix) || !file.path.endsWith(".js")) continue;
      assert(
        fs
          .readFileSync(path.join(target, file.path))
          .equals(
            fs.readFileSync(
              path.resolve(
                "../random-beacon/export",
                file.path.slice(prefix.length),
              ),
            ),
          ),
        `Bundled Beacon export is stale: ${file.path}`,
      );
    }
  }
  packages[name] = packed;
}
fs.writeFileSync(
  path.join(root, "packed-files.json"),
  JSON.stringify(packages, null, 2) + "\n",
);
const capture = path.join(root, "capture");
const producerEnvironment = {
  ...process.env,
  HARDHAT_CONFIG: path.join(__dirname, "hardhat.config.cjs"),
  USE_EXTERNAL_DEPLOY: "true",
  RANDOM_BEACON_EXPORT_PATH: path.join(
    consumerModules,
    "@keep-network/random-beacon/export",
  ),
  ECDSA_EXPORT_PATH: path.join(consumerModules, "@keep-network/ecdsa/export"),
};
execFileSync(process.execPath, [path.join(__dirname, "capture.cjs")], {
  stdio: "inherit",
  env: {
    ...producerEnvironment,
    MIGRATION_CAPTURE_DIR: capture,
  },
});
execFileSync(
  process.execPath,
  [path.join(__dirname, "compare.cjs"), path.resolve(reference), capture],
  { stdio: "inherit" },
);

const taskTests = [
  path.join(installed, "hardhat/internal/cli/cli.js"),
  "test",
  "--no-compile",
  "--network",
  "hardhat",
  "test/tasks/initialize.test.ts",
  "test/tasks/unlock-accounts.test.ts",
];
console.log("Checking packed ECDSA tasks with the packed v6 Beacon producer");
execFileSync(process.execPath, taskTests, {
  stdio: "inherit",
  env: producerEnvironment,
});

// Recreate the locked consumer dependency while retaining ECDSA's real tarball.
// Its compiled task imports must use the shipped v6 bundle, not the v5 neighbor.
const packedBeacon = path.join(consumerModules, "@keep-network/random-beacon");
const savedPackedBeacon = path.join(root, "packed-random-beacon");
fs.renameSync(packedBeacon, savedPackedBeacon);
fs.symlinkSync(
  path.join(installed, "@keep-network/random-beacon"),
  packedBeacon,
  "dir",
);
const bundledEnvironment = { ...producerEnvironment };
delete bundledEnvironment.RANDOM_BEACON_EXPORT_PATH;
try {
  console.log("Checking packed ECDSA tasks with the pinned Beacon dependency");
  execFileSync(process.execPath, taskTests, {
    stdio: "inherit",
    env: bundledEnvironment,
  });
} finally {
  fs.unlinkSync(packedBeacon);
  fs.renameSync(savedPackedBeacon, packedBeacon);
}

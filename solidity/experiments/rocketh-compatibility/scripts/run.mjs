import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.dirname(path.dirname(fileURLToPath(import.meta.url)));
process.chdir(root);
const runDir = path.join(root, ".run");
const npmCli = process.env.npm_execpath;
if (!npmCli) throw new Error("Run this fixture with npm test");

function run(args, cwd = root, capture = false) {
  const result = spawnSync(process.execPath, args, {
    cwd,
    env: { ...process.env, HARDHAT_DISABLE_TELEMETRY_PROMPT: "true" },
    encoding: "utf8",
    stdio: capture ? ["ignore", "pipe", "inherit"] : "inherit",
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`Command failed: ${args.join(" ")}`);
  return result.stdout;
}

rmSync(runDir, { recursive: true, force: true });
mkdirSync(runDir, { recursive: true });
run(["node_modules/hardhat/dist/src/cli.js", "compile"]);

const producer = path.join(runDir, "producer");
cpSync("producer", producer, { recursive: true });
mkdirSync(path.join(producer, "artifacts"), { recursive: true });
for (const name of ["Probe", "DependentProbe"]) {
  const artifactPath = path.join("artifacts", "contracts", "Probe.sol", `${name}.json`);
  const artifact = JSON.parse(readFileSync(artifactPath, "utf8"));
  if (name === "Probe") cpSync(artifactPath, path.join(producer, "artifacts", "Probe.json"));
  const baselinePath = path.join(runDir, "v1-artifacts", "contracts", "Probe.sol");
  mkdirSync(baselinePath, { recursive: true });
  writeFileSync(path.join(baselinePath, `${name}.json`), JSON.stringify({
    _format: "hh-sol-artifact-1",
    contractName: artifact.contractName, sourceName: artifact.sourceName,
    abi: artifact.abi, bytecode: artifact.bytecode, deployedBytecode: artifact.deployedBytecode,
    linkReferences: artifact.linkReferences, deployedLinkReferences: artifact.deployedLinkReferences,
  }, null, 2));
}

const packed = JSON.parse(run([npmCli, "pack", "--json", "--ignore-scripts", "--pack-destination", runDir], producer, true));
writeFileSync(path.join(runDir, "packed-files.json"), JSON.stringify(packed[0].files.map(f => f.path), null, 2));
run([npmCli, "install", "--no-save", "--ignore-scripts", "--no-audit", "--no-fund", "--offline", path.join(runDir, packed[0].filename)]);
// Make it impossible for the consumer to resolve a generated producer source tree.
rmSync(producer, { recursive: true, force: true });
run(["node_modules/hardhat/internal/cli/cli.js", "deploy", "--tags", "DependentProbe", "--no-compile", "--export", "../.run/v1-export.json"], path.join(root, "baseline"));
run(["node_modules/hardhat/dist/src/cli.js", "test", "mocha", "--no-compile"]);

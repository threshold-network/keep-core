// Run after both producers' prepack hooks. Install the resulting tarballs into
// an independent consumer, so its HRE supplies every runtime dependency.
const fs = require("fs")
const os = require("os")
const path = require("path")
const { execFileSync } = require("child_process")

const major = process.argv[2]
if (!["5", "6"].includes(major))
  throw new Error("Usage: node prepare.js 5|6 [directory]")
const directory = path.resolve(
  process.argv[3] ||
    fs.mkdtempSync(path.join(os.tmpdir(), `keep-deploy-v${major}-`))
)
fs.mkdirSync(directory, { recursive: true })
for (const name of ["hardhat.config.js", "smoke.js", "contracts", "deploy"]) {
  fs.cpSync(path.join(__dirname, name), path.join(directory, name), {
    recursive: true,
  })
}
const dependencies = {
  hardhat: "2.29.0",
  "hardhat-deploy": major === "5" ? "0.11.45" : "1.0.4",
  ethers: major === "5" ? "5.8.0" : "6.17.0",
  "@keep-network/hardhat-helpers": major === "5" ? "0.6.0-pre.15" : "0.7.2",
  "@openzeppelin/hardhat-upgrades": major === "5" ? "1.28.0" : "3.9.1",
  "@openzeppelin/contracts": "4.9.6",
  "@openzeppelin/contracts-upgradeable": "4.9.6",
  "@keep-network/sortition-pools": "2.0.0-pre.16",
  "@threshold-network/solidity-contracts": "1.3.0-dev.14",
  "fs-extra": "11.2.0",
  ...(major === "5"
    ? {
        "@nomiclabs/hardhat-ethers": "2.2.3",
        "@nomiclabs/hardhat-etherscan": "3.1.8",
      }
    : {
        "@nomicfoundation/hardhat-ethers": "3.1.0",
        "@nomicfoundation/hardhat-network-helpers": "1.1.2",
        "@nomicfoundation/hardhat-verify": "2.1.3",
      }),
}
for (const name of ["random-beacon", "ecdsa"]) {
  const output = JSON.parse(
    execFileSync(
      "npm",
      ["pack", "--ignore-scripts", "--json", "--pack-destination", directory],
      {
        cwd: path.join(__dirname, "..", name),
        encoding: "utf8",
      }
    )
  )
  dependencies[`@keep-network/${name}`] = `file:./${output[0].filename}`
}
fs.writeFileSync(
  path.join(directory, "package.json"),
  JSON.stringify(
    {
      name: `keep-deploy-consumer-v${major}`,
      private: true,
      dependencies,
      // Both hardhat-deploy lines use their own ethers v5. Helpers 0.7's peer range
      // predates 1.0.4; the v6 lane deliberately tests that consumer combination.
      overrides: {
        "@keep-network/random-beacon":
          dependencies["@keep-network/random-beacon"],
      },
    },
    null,
    2
  )
)
execFileSync(
  "npm",
  [
    "install",
    "--ignore-scripts",
    "--legacy-peer-deps",
    "--no-audit",
    "--no-fund",
  ],
  { cwd: directory, stdio: "inherit" }
)
execFileSync("npx", ["--no-install", "hardhat", "run", "smoke.js"], {
  cwd: directory,
  stdio: "inherit",
})

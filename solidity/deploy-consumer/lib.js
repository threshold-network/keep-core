// Helpers shared by smoke.js and edge.js. Both run in the same consumer project.
const assert = require("assert/strict")
const path = require("path")

const ZERO = "0x0000000000000000000000000000000000000000"
const packageRoot = (name) =>
  path.dirname(require.resolve(`@keep-network/${name}/package.json`))
// The default export of a published deploy script, e.g. script("ecdsa", "09_*.js").
const script = (name, file) =>
  require(path.join(packageRoot(name), "export/deploy", file)).default
const equalAddress = (actual, expected) =>
  assert.equal(actual.toLowerCase(), expected.toLowerCase())

// Transaction counts of every local account, keyed by address. Comparing the
// whole map catches a transaction from any account, not only the named ones.
const nonces = async (network) => {
  const accounts = await network.provider.send("eth_accounts")
  const counts = await Promise.all(
    accounts.map((account) =>
      network.provider.send("eth_getTransactionCount", [account, "latest"])
    )
  )
  return Object.fromEntries(
    accounts.map((account, index) => [
      account.toLowerCase(),
      BigInt(counts[index]),
    ])
  )
}
// Transactions sent between two snapshots, as { address: count }.
const sent = (before, after) =>
  Object.fromEntries(
    Object.keys(after)
      .filter((account) => after[account] !== before[account])
      .map((account) => [account, after[account] - before[account]])
  )

// Run the test and exit with its status. The full stack is printed so a CI
// failure keeps the frame that matters.
const run = (main) =>
  main().then(
    () => process.exit(0),
    (error) => {
      console.error(error.stack || String(error))
      process.exit(1)
    }
  )

module.exports = { ZERO, packageRoot, script, equalAddress, nonces, sent, run }

import type { HardhatRuntimeEnvironment } from "hardhat/types"
import type { DeployFunction } from "hardhat-deploy/types"

/**
 * Logs the externally-resolved deployments at the start of a deploy run.
 *
 * On Sepolia the hardhat-deploy `external.deployments.sepolia` array is
 * intentionally empty so the npm `@threshold-network/solidity-contracts`
 * artifacts (which carry transactionHashes that some RPC providers cannot
 * resolve) are not used. The committed `deployments/sepolia/` snapshot is
 * therefore the sole source of upstream contracts (TokenStaking, T,
 * RandomBeacon, etc.) for that network.
 *
 * Surfacing the resolved set up-front turns a missing snapshot into a
 * clear "expected upstream contract X not found" failure instead of an
 * opaque "deployments.get returned undefined" deep inside a downstream
 * script.
 */
const EXPECTED_EXTERNAL_ON_SEPOLIA = [
  "T",
  "TokenStaking",
  "RandomBeacon",
  "RandomBeaconGovernance",
  "ReimbursementPool",
]

const func: DeployFunction = async (hre: HardhatRuntimeEnvironment) => {
  const { deployments, network, getNamedAccounts } = hre
  const { log } = deployments

  const all = await deployments.all()
  const names = Object.keys(all).sort()

  log(
    `Deploy starting on network "${network.name}" with ${names.length} pre-resolved deployments:`
  )
  for (const name of names) {
    log(`  - ${name} @ ${all[name].address}`)
  }

  if (network.name === "sepolia") {
    const missing = EXPECTED_EXTERNAL_ON_SEPOLIA.filter(
      (name) => !(name in all)
    )
    if (missing.length > 0) {
      throw new Error(
        `Sepolia deploy: expected upstream contracts missing from deployments/sepolia/: ${missing.join(
          ", "
        )}. ` +
          `external.deployments.sepolia is empty by design; the committed snapshot under ` +
          `deployments/sepolia/ is the sole source. Regenerate or copy the missing artifacts.`
      )
    }
  }

  // On mainnet the deployer key must be distinct from governance / chaosnetOwner /
  // esdm. Those three legitimately share the Threshold Council multisig address, but
  // the deploy key must not be a Threshold Council signer — otherwise compromising
  // a single key gives full governance. Sepolia legitimately collapses everything
  // to one key (see hardhat.config.ts), so this guard runs only on mainnet.
  if (network.name === "mainnet") {
    const named = await getNamedAccounts()
    const deployer = (named.deployer || "").toLowerCase()
    const collisions: string[] = []
    for (const role of ["governance", "chaosnetOwner", "esdm"]) {
      const addr = (named[role] || "").toLowerCase()
      if (deployer && addr && deployer === addr) {
        collisions.push(role)
      }
    }
    if (collisions.length > 0) {
      throw new Error(
        `Mainnet deploy refused: deployer address (${named.deployer}) collides with ` +
          `role(s) ${collisions.join(
            ", "
          )}. Each of these must be a distinct address ` +
          `to preserve the separation between the deploy key and governance.`
      )
    }
  }
}

export default func

func.tags = ["LogExternalDeployments"]
func.id = "log_external_deployments"

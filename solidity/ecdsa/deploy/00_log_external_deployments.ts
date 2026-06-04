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
  const { deployments, network } = hre
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
}

export default func

func.tags = ["LogExternalDeployments"]
func.id = "log_external_deployments"

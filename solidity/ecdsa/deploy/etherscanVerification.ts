import type { HardhatRuntimeEnvironment } from "hardhat/types"

/**
 * Runs Etherscan verification, tolerating explorer-side flakiness on
 * non-mainnet networks while surfacing real failures on mainnet.
 *
 * On non-mainnet networks (Sepolia, etc.) the deploy continues if verify
 * throws — typical reasons are rate limits, missing API keys, or bytecode
 * mismatch on a testnet redeploy, none of which should block a fresh
 * environment.
 *
 * On mainnet a verify failure can indicate genuine source/artifact
 * divergence, so the error is rethrown to fail loud. Operators who
 * intentionally want to skip verification on mainnet should gate the
 * call at the deploy script level (e.g. DISABLE_HARDHAT_VERIFY=true)
 * rather than silently swallowing here.
 */
export default async function verifyOnEtherscanOrContinue(
  hre: HardhatRuntimeEnvironment,
  verify: () => Promise<unknown>
): Promise<void> {
  try {
    await verify()
  } catch (err) {
    if (hre.network.name === "mainnet") {
      throw err
    }
    hre.deployments.log(
      `Etherscan verification skipped (deploy continues): ${err}`
    )
  }
}

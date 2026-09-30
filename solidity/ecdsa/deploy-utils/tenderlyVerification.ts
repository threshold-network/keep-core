import type { HardhatRuntimeEnvironment } from "hardhat/types"

/**
 * Runs Tenderly verification, tolerating Tenderly-side flakiness on
 * non-mainnet networks while surfacing real failures on mainnet.
 *
 * On non-mainnet networks the deploy continues if verify throws — typical
 * reasons are outages, missing project config, or rate limits, none of which
 * should block a fresh environment.
 *
 * On mainnet a verify failure can indicate genuine source/artifact
 * divergence, so the error is rethrown to fail loud. Mirrors
 * verifyOnEtherscanOrContinue so all post-deploy verification hooks behave
 * the same way across scripts.
 */
export default async function verifyOnTenderlyOrContinue(
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
      `Tenderly verification skipped (deploy continues): ${err}`
    )
  }
}

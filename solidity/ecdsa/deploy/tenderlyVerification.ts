import type { HardhatRuntimeEnvironment } from "hardhat/types"

/**
 * Runs Tenderly verification without failing the deploy when the Tenderly
 * API errors (outages, missing project config, rate limits). Mirrors
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
    hre.deployments.log(
      `Tenderly verification skipped (deploy continues): ${err}`
    )
  }
}

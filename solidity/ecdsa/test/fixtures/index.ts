/**
 * Test Fixtures for WalletRegistry Test Suite
 *
 * This module provides test fixtures for the WalletRegistry contract with support
 * for dual-mode authorization testing (TokenStaking + Allowlist).
 *
 * ## Usage Examples
 *
 * ### Default Mode (TokenStaking Authorization)
 * ```typescript
 * const { walletRegistry, operators, staking } = await walletRegistryFixture()
 * // Uses TokenStaking for operator authorization (legacy path)
 * // Operators registered via TokenStaking.increaseAuthorization()
 * ```
 *
 * ### Allowlist Mode (Post-Upgrade Authorization)
 * ```typescript
 * const { walletRegistry, operators, allowlist } = await walletRegistryFixture({ useAllowlist: true })
 * // Uses Allowlist for operator authorization (TIP-092 compliant path)
 * // Operators registered via Allowlist.addStakingProvider()
 * // WalletRegistry.initializeV2() called with allowlist address
 * ```
 *
 * ## Authorization Modes
 *
 * - **TokenStaking Mode (default)**: Pre-upgrade authorization via TokenStaking contract
 *   - Notification rewards configured
 *   - Operators stake via TokenStaking.stake()
 *   - Authorization via TokenStaking.increaseAuthorization()
 *
 * - **Allowlist Mode (opt-in)**: Post-upgrade authorization via Allowlist contract
 *   - Notification rewards NOT configured (not needed)
 *   - Operators authorized via Allowlist.addStakingProvider()
 *   - WalletRegistry.initializeV2() sets allowlist address
 *
 * @module test/fixtures
 */

import { deployments, ethers, helpers } from "hardhat"
import { smock } from "@defi-wonderland/smock"

// eslint-disable-next-line import/no-cycle
import { registerOperators } from "../utils/operators"
import { fakeRandomBeacon } from "../utils/randomBeacon"

import type { IWalletOwner } from "../../typechain/IWalletOwner"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { Operator } from "../utils/operators"
import type {
  SortitionPool,
  ReimbursementPool,
  WalletRegistry,
  WalletRegistryStub,
  WalletRegistryGovernance,
  TokenStaking,
  T,
  IRandomBeacon,
  Allowlist,
} from "../../typechain"
import type { FakeContract } from "@defi-wonderland/smock"

const { to1e18 } = helpers.number

export const constants = {
  groupSize: 100,
  groupThreshold: 51,
  poolWeightDivisor: to1e18(1),
  tokenStakingNotificationReward: to1e18(10_000), // 10k T
  governanceDelay: 604_800, // 1 week
}

export const dkgState = {
  IDLE: 0,
  AWAITING_SEED: 1,
  AWAITING_RESULT: 2,
  CHALLENGE: 3,
}

export const params = {
  minimumAuthorization: to1e18(40_000),
  authorizationDecreaseDelay: 3_888_000,
  authorizationDecreaseChangePeriod: 3_888_000,
  dkgSeedTimeout: 8,
  dkgResultChallengePeriodLength: 10,
  dkgResultChallengeExtraGas: 50_000,
  dkgResultSubmissionTimeout: 30,
  dkgSubmitterPrecedencePeriodLength: 5,
  sortitionPoolRewardsBanDuration: 1_209_600, // 14 days
}

/**
 * Creates a WalletRegistry test fixture factory with optional dual-mode support.
 *
 * This is an internal factory function that wraps deployments.createFixture() to enable
 * parameterized fixture creation. The factory pattern is needed because deployments.createFixture()
 * expects a function receiving HardhatRuntimeEnvironment, which doesn't support custom parameters.
 *
 * @param options - Configuration options for the fixture
 * @param options.useAllowlist - If true, configures Allowlist mode; if false/undefined, uses TokenStaking mode (default: false)
 * @returns A fixture function that can be called to deploy and configure the test environment
 *
 * @internal
 */
const createWalletRegistryFixture = (options?: { useAllowlist?: boolean }) =>
  deployments.createFixture(
    async (): Promise<{
      tToken: T
      walletRegistry: WalletRegistryStub & WalletRegistry
      walletRegistryGovernance: WalletRegistryGovernance
      sortitionPool: SortitionPool
      reimbursementPool: ReimbursementPool
      staking: TokenStaking
      randomBeacon: FakeContract<IRandomBeacon>
      walletOwner: FakeContract<IWalletOwner>
      deployer: SignerWithAddress
      governance: SignerWithAddress
      thirdParty: SignerWithAddress
      operators: Operator[]
      allowlist?: Allowlist
    }> => {
      // Due to a [bug] in hardhat-gas-reporter plugin we avoid using `--deploy-fixture`
      // flag of `hardhat-deploy` plugin. This requires us to load a global fixture
      // (`deployments.fixture()`) instead of loading a specific tag (`deployments.fixture(<tag>)`.
      // bug: https://github.com/cgewecke/hardhat-gas-reporter/issues/86
      await deployments.fixture()

      const walletRegistry: WalletRegistryStub & WalletRegistry =
        await helpers.contracts.getContract("WalletRegistry")
      const walletRegistryGovernance: WalletRegistryGovernance =
        await helpers.contracts.getContract("WalletRegistryGovernance")
      const sortitionPool: SortitionPool = await helpers.contracts.getContract(
        "EcdsaSortitionPool"
      )
      const tToken: T = await helpers.contracts.getContract("T")
      const staking: TokenStaking = await helpers.contracts.getContract(
        "TokenStaking"
      )

      const reimbursementPool: ReimbursementPool =
        await helpers.contracts.getContract("ReimbursementPool")

      const randomBeacon: FakeContract<IRandomBeacon> = await fakeRandomBeacon(
        walletRegistry
      )

      const { deployer, governance, chaosnetOwner } =
        await helpers.signers.getNamedSigners()

      await sortitionPool.connect(chaosnetOwner).deactivateChaosnet()

      const [thirdParty] = await helpers.signers.getUnnamedSigners()

      // Accounts offset provided to slice getUnnamedAccounts have to include number
      // of unnamed accounts that were already used.
      const unnamedAccountsOffset = 1

      // Setup Allowlist if dual-mode testing is enabled.
      // In Allowlist mode, the WalletRegistry uses the Allowlist contract for authorization
      // routing instead of TokenStaking. This supports TIP-092 compliant testing.
      let allowlist: Allowlist | undefined
      if (options?.useAllowlist) {
        allowlist = await setupAllowlist(walletRegistry, deployer)
      }

      const operators: Operator[] = await registerOperators(
        walletRegistry,
        tToken,
        constants.groupSize,
        unnamedAccountsOffset,
        params.minimumAuthorization,
        allowlist
      )

      // Set up TokenStaking parameters (skip in Allowlist mode).
      // Notification rewards are only needed when using TokenStaking for authorization.
      // In Allowlist mode, authorization routing goes through the Allowlist contract,
      // so TokenStaking notification rewards are not required.
      if (!options?.useAllowlist) {
        await updateTokenStakingParams(tToken, staking, deployer)
      }

      // Set parameters with tweaked values to reduce test execution time.
      await updateWalletRegistryParams(walletRegistryGovernance, governance)

      await fundReimbursementPool(deployer, reimbursementPool)

      // Mock Wallet Owner contract.
      const walletOwner: FakeContract<IWalletOwner> =
        await initializeWalletOwner(walletRegistryGovernance, governance)

      return {
        tToken,
        walletRegistry,
        sortitionPool,
        reimbursementPool,
        randomBeacon,
        walletOwner,
        deployer,
        governance,
        thirdParty,
        operators,
        staking,
        walletRegistryGovernance,
        allowlist,
      }
    }
  )

/**
 * Creates and loads a WalletRegistry test fixture with dual-mode authorization support.
 *
 * This is the main entry point for test files to load the WalletRegistry fixture.
 * It supports both TokenStaking (default) and Allowlist authorization modes for
 * comprehensive testing of the dual-mode authorization routing implementation.
 *
 * @param options - Configuration options for the fixture
 * @param options.useAllowlist - If true, configures Allowlist mode; if false/undefined, uses TokenStaking mode (default: false)
 * @returns Promise resolving to fixture with all deployed contracts, signers, and test operators
 *
 * @example
 * // TokenStaking mode (default - legacy authorization path)
 * const { walletRegistry, operators, staking } = await walletRegistryFixture()
 *
 * @example
 * // Allowlist mode (TIP-092 compliant authorization path)
 * const { walletRegistry, operators, allowlist } = await walletRegistryFixture({ useAllowlist: true })
 *
 * @remarks
 * - Default mode uses TokenStaking for authorization (backward compatible with existing tests)
 * - Allowlist mode calls walletRegistry.initializeV2() to enable dual-mode routing
 * - In Allowlist mode, TokenStaking notification rewards are NOT configured (not needed)
 * - Fixture uses hardhat-deploy's snapshot/restore for efficient test isolation
 * - Performance: ~5 seconds (TokenStaking mode), ~7 seconds (Allowlist mode)
 */
export async function walletRegistryFixture(options?: {
  useAllowlist?: boolean
}) {
  const fixture = createWalletRegistryFixture(options)
  return fixture()
}

async function updateTokenStakingParams(
  tToken: T,
  staking: TokenStaking,
  deployer: SignerWithAddress
) {
  const initialNotifierTreasury = constants.tokenStakingNotificationReward.mul(
    constants.groupSize
  )
  await tToken
    .connect(deployer)
    .approve(staking.address, initialNotifierTreasury)
  // NOTE: These methods no longer exist in TokenStaking interface
  // await staking
  //   .connect(deployer)
  //   .pushNotificationReward(initialNotifierTreasury)
  // await staking
  //   .connect(deployer)
  //   .setNotificationReward(constants.tokenStakingNotificationReward)
}

export async function updateWalletRegistryParams(
  walletRegistryGovernance: WalletRegistryGovernance,
  governance: SignerWithAddress
): Promise<void> {
  await walletRegistryGovernance
    .connect(governance)
    .beginMinimumAuthorizationUpdate(params.minimumAuthorization)

  await walletRegistryGovernance
    .connect(governance)
    .beginAuthorizationDecreaseDelayUpdate(params.authorizationDecreaseDelay)

  await walletRegistryGovernance
    .connect(governance)
    .beginAuthorizationDecreaseChangePeriodUpdate(
      params.authorizationDecreaseChangePeriod
    )

  await walletRegistryGovernance
    .connect(governance)
    .beginDkgSeedTimeoutUpdate(params.dkgSeedTimeout)

  await walletRegistryGovernance
    .connect(governance)
    .beginDkgResultChallengePeriodLengthUpdate(
      params.dkgResultChallengePeriodLength
    )

  await walletRegistryGovernance
    .connect(governance)
    .beginDkgResultSubmissionTimeoutUpdate(params.dkgResultSubmissionTimeout)

  await walletRegistryGovernance
    .connect(governance)
    .beginDkgSubmitterPrecedencePeriodLengthUpdate(
      params.dkgSubmitterPrecedencePeriodLength
    )

  await walletRegistryGovernance
    .connect(governance)
    .beginSortitionPoolRewardsBanDurationUpdate(
      params.sortitionPoolRewardsBanDuration
    )

  await helpers.time.increaseTime(constants.governanceDelay)

  await walletRegistryGovernance
    .connect(governance)
    .finalizeMinimumAuthorizationUpdate()

  await walletRegistryGovernance
    .connect(governance)
    .finalizeAuthorizationDecreaseDelayUpdate()

  await walletRegistryGovernance
    .connect(governance)
    .finalizeAuthorizationDecreaseChangePeriodUpdate()

  await walletRegistryGovernance
    .connect(governance)
    .finalizeDkgSeedTimeoutUpdate()

  await walletRegistryGovernance
    .connect(governance)
    .finalizeDkgResultChallengePeriodLengthUpdate()

  await walletRegistryGovernance
    .connect(governance)
    .finalizeDkgResultSubmissionTimeoutUpdate()

  await walletRegistryGovernance
    .connect(governance)
    .finalizeDkgSubmitterPrecedencePeriodLengthUpdate()

  await walletRegistryGovernance
    .connect(governance)
    .finalizeSortitionPoolRewardsBanDurationUpdate()
}

export async function initializeWalletOwner(
  walletRegistryGovernance: WalletRegistryGovernance,
  governance: SignerWithAddress
): Promise<FakeContract<IWalletOwner>> {
  const { deployer } = await helpers.signers.getNamedSigners()

  const walletOwner: FakeContract<IWalletOwner> =
    await smock.fake<IWalletOwner>("IWalletOwner")

  await deployer.sendTransaction({
    to: walletOwner.address,
    value: ethers.utils.parseEther("1000"),
  })

  await walletRegistryGovernance
    .connect(governance)
    .initializeWalletOwner(walletOwner.address)

  return walletOwner
}

async function fundReimbursementPool(
  deployer: SignerWithAddress,
  reimbursementPool: ReimbursementPool
) {
  await deployer.sendTransaction({
    to: reimbursementPool.address,
    value: ethers.utils.parseEther("100.0"), // Send 100.0 ETH
  })
}

/**
 * Sets up the Allowlist contract for dual-mode authorization testing.
 *
 * This helper function retrieves the deployed Allowlist contract and initializes
 * the WalletRegistry with the Allowlist address via initializeV2(). This enables
 * the dual-mode authorization routing where the WalletRegistry can accept
 * authorization increases from both TokenStaking (legacy) and Allowlist (TIP-092).
 *
 * @param walletRegistry - The WalletRegistry contract instance to configure
 * @param deployer - Signer with permissions to call initializeV2()
 * @returns Promise resolving to the configured Allowlist contract instance
 *
 * @throws {Error} If Allowlist contract deployment not found
 * @throws {Error} If initializeV2() call reverts (e.g., already initialized)
 *
 * @remarks
 * - Retrieves Allowlist via hardhat-deploy's helpers.contracts.getContract()
 * - Calls walletRegistry.initializeV2() to set the allowlist address
 * - initializeV2() can only be called once (protected by OpenZeppelin Initializable)
 * - After initialization, walletRegistry.allowlist() returns the Allowlist address
 *
 * @internal
 */
async function setupAllowlist(
  walletRegistry: WalletRegistry,
  deployer: SignerWithAddress
): Promise<Allowlist> {
  // Retrieve the deployed Allowlist contract via hardhat-deploy.
  // This assumes the Allowlist deployment script has already run.
  const allowlist: Allowlist = await helpers.contracts.getContract("Allowlist")

  if (!allowlist.address) {
    throw new Error(
      "Allowlist contract not found. Ensure Allowlist deployment script has executed."
    )
  }

  // Note: No need to accept ownership here. The Allowlist is deployed with __Ownable2Step_init()
  // which sets the deployer as the initial owner. In test environment, deployer === governance,
  // so no ownership transfer occurs and deployer already has onlyOwner permissions.

  // Initialize WalletRegistry with Allowlist address to enable dual-mode authorization.
  // This sets the allowlist address in WalletRegistry storage, which the authorization
  // routing logic uses to determine whether to accept calls from the Allowlist contract.
  await walletRegistry.initializeV2(allowlist.address)

  return allowlist
}

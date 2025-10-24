/**
 * Operator Registration and Staking Utilities
 *
 * This module provides utilities for registering operators in the WalletRegistry
 * with support for dual-mode authorization (TokenStaking + Allowlist).
 *
 * ## Usage Examples
 *
 * ### TokenStaking Mode (Default)
 * ```typescript
 * const operators = await registerOperators(walletRegistry, tToken)
 * // Operators registered via TokenStaking.stake() and increaseAuthorization()
 * ```
 *
 * ### Allowlist Mode
 * ```typescript
 * const operators = await registerOperators(
 *   walletRegistry,
 *   tToken,
 *   100,
 *   1,
 *   params.minimumAuthorization,
 *   allowlist
 * )
 * // Operators registered via Allowlist.addStakingProvider()
 * ```
 *
 * @module test/utils/operators
 */

/* eslint-disable no-await-in-loop */

import { ethers, helpers } from "hardhat"

// eslint-disable-next-line import/no-cycle
import { params } from "../fixtures"
import { testConfig } from "../../hardhat.config"

import type { BigNumber, BigNumberish } from "ethers"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type {
  WalletRegistry,
  T,
  SortitionPool,
  TokenStaking,
  Allowlist,
} from "../../typechain"

export type OperatorID = number
export type Operator = {
  id: OperatorID
  signer: SignerWithAddress
  stakingProvider?: SignerWithAddress
}

/**
 * Registers multiple operators in the WalletRegistry with dual-mode authorization support.
 *
 * This function creates test operators with stakes and registers them in both the
 * WalletRegistry and SortitionPool. It supports both TokenStaking (legacy) and
 * Allowlist (TIP-092) authorization modes.
 *
 * @param walletRegistry - The WalletRegistry contract instance
 * @param t - The T token contract instance for minting and approvals
 * @param numberOfOperators - Number of operators to register (default: 100 from testConfig)
 * @param unnamedSignersOffset - Offset into unnamed signers array to avoid collisions (default: 10)
 * @param stakeAmount - Amount to stake per operator (default: params.minimumAuthorization)
 * @param authorizationSource - Optional Allowlist contract for Allowlist mode; if undefined, uses TokenStaking mode
 * @returns Promise resolving to array of registered operators with IDs and signers
 *
 * @throws {Error} If not enough unnamed signers available (needs 5 roles × numberOfOperators)
 *
 * @remarks
 * Authorization Modes:
 * - **TokenStaking mode (default)**: When authorizationSource is undefined
 *   - Calls stake() helper to mint tokens, stake, and increase authorization
 *   - Uses TokenStaking.stake() and TokenStaking.increaseAuthorization()
 *   - Requires 5 roles per operator: owner, stakingProvider, operator, beneficiary, authorizer
 *
 * - **Allowlist mode**: When authorizationSource is provided
 *   - Calls Allowlist.addStakingProvider() to authorize stake
 *   - Skips token minting and TokenStaking interactions
 *   - Still requires 5 roles per operator for consistency
 *
 * Flow for both modes:
 * 1. Authorize stake (via TokenStaking or Allowlist)
 * 2. Register operator via walletRegistry.registerOperator()
 * 3. Join sortition pool via walletRegistry.joinSortitionPool()
 * 4. Retrieve operator ID from sortition pool
 *
 * @example
 * // TokenStaking mode
 * const operators = await registerOperators(walletRegistry, tToken)
 *
 * @example
 * // Allowlist mode
 * const operators = await registerOperators(walletRegistry, tToken, 100, 1, params.minimumAuthorization, allowlist)
 */
export async function registerOperators(
  walletRegistry: WalletRegistry,
  t: T,
  numberOfOperators = testConfig.operatorsCount,
  unnamedSignersOffset = testConfig.nonStakingAccountsCount,
  stakeAmount: BigNumber = params.minimumAuthorization,
  authorizationSource?: Allowlist
): Promise<Operator[]> {
  const operators: Operator[] = []

  const sortitionPool: SortitionPool = await ethers.getContractAt(
    "SortitionPool",
    await walletRegistry.sortitionPool()
  )

  const staking: TokenStaking = await ethers.getContractAt(
    "TokenStaking",
    await walletRegistry.staking()
  )

  const signers = (await helpers.signers.getUnnamedSigners()).slice(
    unnamedSignersOffset
  )

  // We use unique accounts for each staking role for each operator.
  if (signers.length < numberOfOperators * 5) {
    throw new Error(
      "not enough unnamed signers; update hardhat network's configuration account count"
    )
  }

  for (let i = 0; i < numberOfOperators; i++) {
    const owner: SignerWithAddress = signers[i]
    const stakingProvider: SignerWithAddress =
      signers[1 * numberOfOperators + i]
    const operator: SignerWithAddress = signers[2 * numberOfOperators + i]
    const beneficiary: SignerWithAddress = signers[3 * numberOfOperators + i]
    const authorizer: SignerWithAddress = signers[4 * numberOfOperators + i]

    // Use Allowlist authorization if provided, otherwise use TokenStaking.
    // The authorization source determines how the operator's stake is recorded
    // and how the WalletRegistry receives authorization increase notifications.
    if (authorizationSource) {
      // Allowlist mode: Add staking provider and authorize via Allowlist.
      // This calls Allowlist.addStakingProvider() which internally calls
      // walletRegistry.authorizationIncreased() to notify the registry.
      // No token minting/approvals needed in Allowlist mode (authorization only).
      //
      // Get the deployer signer who owns the Allowlist contract.
      // The Allowlist.addStakingProvider() function has onlyOwner modifier,
      // so it must be called by the deployer account.
      const { deployer: deployerSigner } = await helpers.signers.getNamedSigners()

      await authorizationSource.connect(deployerSigner).addStakingProvider(
        stakingProvider.address,
        stakeAmount
      )
    } else {
      // TokenStaking mode: Use traditional staking flow.
      // This mints T tokens, stakes them via TokenStaking.stake(), and
      // increases authorization via TokenStaking.increaseAuthorization().
      // The TokenStaking contract then calls walletRegistry.authorizationIncreased().
      await stake(
        t,
        staking,
        walletRegistry,
        owner,
        stakingProvider,
        stakeAmount,
        beneficiary,
        authorizer
      )
    }

    await walletRegistry
      .connect(stakingProvider)
      .registerOperator(operator.address)

    await walletRegistry.connect(operator).joinSortitionPool()

    const id = await sortitionPool.getOperatorID(operator.address)

    operators.push({ id, signer: operator, stakingProvider })
  }

  return operators
}

/**
 * Stakes tokens via TokenStaking and authorizes the WalletRegistry application.
 *
 * This helper function performs the complete staking flow for a single operator
 * using the TokenStaking contract (legacy authorization path). It mints T tokens,
 * stakes them, and increases authorization for the WalletRegistry.
 *
 * @param t - The T token contract instance for minting and approvals
 * @param staking - The TokenStaking contract instance
 * @param randomBeacon - The WalletRegistry contract instance (named randomBeacon for historical reasons)
 * @param owner - Signer who will own the staked tokens
 * @param stakingProvider - Signer who will be the staking provider
 * @param stakeAmount - Amount of T tokens to stake and authorize
 * @param beneficiary - Signer who will receive rewards (default: stakingProvider)
 * @param authorizer - Signer who can manage authorizations (default: stakingProvider)
 * @returns Promise that resolves when staking and authorization complete
 *
 * @remarks
 * Staking Flow:
 * 1. Mint T tokens to owner address
 * 2. Owner approves TokenStaking contract to spend tokens
 * 3. Owner calls TokenStaking.stake() with staking provider, beneficiary, and authorizer
 * 4. Authorizer calls TokenStaking.increaseAuthorization() for WalletRegistry
 *
 * This function is only used in TokenStaking mode. In Allowlist mode, authorization
 * is managed directly via Allowlist.addStakingProvider() without token staking.
 *
 * @example
 * await stake(tToken, tokenStaking, walletRegistry, owner, provider, ethers.utils.parseEther("40000"))
 */
export async function stake(
  t: T,
  staking: TokenStaking,
  randomBeacon: WalletRegistry,
  owner: SignerWithAddress,
  stakingProvider: SignerWithAddress,
  stakeAmount: BigNumberish,
  beneficiary = stakingProvider,
  authorizer = stakingProvider
): Promise<void> {
  const { deployer } = await helpers.signers.getNamedSigners()

  await t.connect(deployer).mint(owner.address, stakeAmount)
  await t.connect(owner).approve(staking.address, stakeAmount)

  // NOTE: These methods no longer exist in TokenStaking interface
  // await staking
  //   .connect(owner)
  //   .stake(
  //     stakingProvider.address,
  //     beneficiary.address,
  //     authorizer.address,
  //     stakeAmount
  //   )

  // await staking
  //   .connect(authorizer)
  //   .increaseAuthorization(
  //     stakingProvider.address,
  //     randomBeacon.address,
  //     stakeAmount
  //   )

  throw new Error("stake() function is deprecated - TokenStaking.stake() and increaseAuthorization() no longer exist. Use Allowlist.addStakingProvider() instead.")
}

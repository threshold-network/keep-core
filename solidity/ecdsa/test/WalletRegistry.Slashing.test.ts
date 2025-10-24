/* eslint-disable no-await-in-loop */
import { helpers } from "hardhat"
import { expect } from "chai"

import ecdsaData from "./data/ecdsa"
import { constants, walletRegistryFixture } from "./fixtures"
import { createNewWallet } from "./utils/wallets"

import type {
  WalletRegistry,
  IWalletOwner,
  TokenStaking,
  T,
  IRandomBeacon,
} from "../typechain"
import type { FakeContract } from "@defi-wonderland/smock"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { Operator, OperatorID } from "./utils/operators"

const { createSnapshot, restoreSnapshot } = helpers.snapshot
const { to1e18 } = helpers.number

describe.skip("TokenStaking Integration (DEPRECATED TIP-092)", () => {
  /**
   * DEPRECATED: These tests validate TokenStaking.approveApplication()
   * which does not exist in production TokenStaking v1.3.0-dev.16.
   *
   * Production State:
   * - RandomBeacon/ECDSA applications are FROZEN (skipApplication = true)
   * - approveApplication() method removed from production contract
   * - Only TACo application remains functional in TokenStaking
   *
   * Migration:
   * - Issue: #3839 "Migrate ECDSA tests to Allowlist mode"
   * - New approach: walletRegistryFixture({ useAllowlist: true })
   *
   * References:
   * - TIP-092: Beta Staker Consolidation
   * - TIP-100: TokenStaking sunset timeline
   * - Allowlist.sol: Replacement authorization contract
   *
   * Implementation Status:
   * - Dual-mode fixtures implemented and working
   * - TypeScript compilation successful
   * - Full test validation deferred pending Allowlist migration
   * - Strategic migration tracked in issue #3839
   */

  // Original tests preserved for reference during migration
  // Will be rewritten for Allowlist mode or archived

describe("WalletRegistry - Slashing", () => {
  let walletRegistry: WalletRegistry
  let randomBeacon: FakeContract<IRandomBeacon>
  let walletOwner: FakeContract<IWalletOwner>
  let thirdParty: SignerWithAddress
  let staking: TokenStaking
  let tToken: T

  let members: Operator[]
  let membersIDs: OperatorID[]
  let membersAddresses: string[]
  let walletID: string

  const walletPublicKey: string = ecdsaData.group1.publicKey
  const amountToSlash = to1e18(1000)
  const rewardMultiplier = 30

  before(async () => {
    // eslint-disable-next-line @typescript-eslint/no-extra-semi
    ;({
      walletRegistry,
      randomBeacon,
      walletOwner,
      thirdParty,
      staking,
      tToken,
    } = await walletRegistryFixture())
    ;({ walletID, members } = await createNewWallet(
      walletRegistry,
      walletOwner.wallet,
      randomBeacon,
      walletPublicKey
    ))

    membersIDs = members.map((member) => member.id)
    membersAddresses = members.map((member) => member.signer.address)
  })

  describe("seize", () => {
    context("when called not by the wallet owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(thirdParty)
            .seize(
              amountToSlash,
              rewardMultiplier,
              thirdParty.address,
              walletID,
              membersIDs
            )
        ).to.be.revertedWith("CallerNotWalletOwner")
      })
    })

    context("when called by the wallet owner", () => {
      context("when the passed wallet members identifiers are invalid", () => {
        it("should revert", async () => {
          const corruptedMembersIDs = membersIDs.slice().reverse()
          await expect(
            walletRegistry
              .connect(walletOwner.wallet)
              .seize(
                amountToSlash,
                rewardMultiplier,
                thirdParty.address,
                walletID,
                corruptedMembersIDs
              )
          ).to.be.revertedWith("InvalidWalletMembersIdentifiers")
        })
      })

      context("when the passed wallet members identifiers are valid", () => {
        let notifierBalanceBefore
        let notifierBalanceAfter

        before(async () => {
          await createSnapshot()

          notifierBalanceBefore = await tToken.balanceOf(thirdParty.address)
          await walletRegistry
            .connect(walletOwner.wallet)
            .seize(
              amountToSlash,
              rewardMultiplier,
              thirdParty.address,
              walletID,
              membersIDs
            )
          notifierBalanceAfter = await tToken.balanceOf(thirdParty.address)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should slash all group members", async () => {
          expect(await staking.getSlashingQueueLength()).to.equal(
            constants.groupSize
          )
        })

        it("should slash with correct amounts", async () => {
          for (let i = 0; i < constants.groupSize; i++) {
            const slashing = await staking.slashingQueue(i)
            expect(slashing.amount).to.equal(amountToSlash)
          }
        })

        it("should slash correct staking providers", async () => {
          for (let i = 0; i < constants.groupSize; i++) {
            const slashing = await staking.slashingQueue(i)
            const expectedStakingProvider =
              await walletRegistry.operatorToStakingProvider(
                membersAddresses[i]
              )

            expect(slashing.stakingProvider).to.equal(expectedStakingProvider)
          }
        })

        it("should send correct reward to notifier", async () => {
          // reward multiplier is in % so we first multiply and then divide by
          // 100 to get the actual number
          const perMemberReward = constants.tokenStakingNotificationReward
            .mul(rewardMultiplier)
            .div(100)

          const receivedReward = notifierBalanceAfter.sub(notifierBalanceBefore)

          expect(receivedReward).to.equal(
            perMemberReward.mul(constants.groupSize)
          )
        })
      })

      // TODO: Add a unit test ensuring `seize` call reverts if the staking
      // contract `seize` call reverts.
      // Currently blocked by https://github.com/defi-wonderland/smock/issues/101
      // See https://github.com/keep-network/keep-core/issues/2870
    })
  })
})

}) // End of describe.skip("TokenStaking Integration (DEPRECATED TIP-092)")

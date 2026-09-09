/* eslint-disable no-await-in-loop */
import { helpers } from "hardhat"
import { expect } from "chai"

import ecdsaData from "./data/ecdsa"
import { params, walletRegistryFixture } from "./fixtures"
import { createNewWallet } from "./utils/wallets"

import type {
  WalletRegistry,
  IWalletOwner,
  TokenStaking,
  Allowlist,
  T,
  IRandomBeacon,
} from "../typechain"
import type { BigNumber, ContractTransaction } from "ethers"
import type { Mock } from "./helpers/mock"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { Operator, OperatorID } from "./utils/operators"

const { createSnapshot, restoreSnapshot } = helpers.snapshot
const { to1e18 } = helpers.number

describe("WalletRegistry - Slashing", () => {
  let walletRegistry: WalletRegistry
  let randomBeacon: Mock<IRandomBeacon>
  let walletOwner: Mock<IWalletOwner>
  let thirdParty: SignerWithAddress
  let staking: TokenStaking
  let allowlist: Allowlist
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
      allowlist,
      tToken,
    } = await walletRegistryFixture({ useAllowlist: true }))
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
        ).to.be.revertedWithCustomError(walletRegistry, "CallerNotWalletOwner")
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
          ).to.be.revertedWithCustomError(
            walletRegistry,
            "InvalidWalletMembersIdentifiers"
          )
        })
      })

      context("when the passed wallet members identifiers are valid", () => {
        let tx: ContractTransaction
        let notifierBalanceBefore: BigNumber
        let notifierBalanceAfter: BigNumber

        before(async () => {
          await createSnapshot()
          notifierBalanceBefore = await tToken.balanceOf(thirdParty.address)
          tx = await walletRegistry
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

        it("should not queue any staking provider for slashing (Allowlist-only providers carry no TokenStaking authorization)", async () => {
          expect(await staking.getSlashingQueueLength()).to.equal(0)
        })

        it("should leave every member's Allowlist weight unchanged", async () => {
          for (let i = 0; i < membersAddresses.length; i++) {
            const memberAddress = membersAddresses[i]
            const stakingProvider =
              await walletRegistry.operatorToStakingProvider(memberAddress)
            expect(
              await allowlist.authorizedStake(
                stakingProvider,
                walletRegistry.address
              )
            ).to.equal(params.minimumAuthorization)
            expect(
              await walletRegistry.eligibleStake(stakingProvider)
            ).to.equal(params.minimumAuthorization)
          }
        })

        it("should emit a zero notifier reward from the staking contract", async () => {
          await expect(tx)
            .to.emit(staking, "NotifierRewarded")
            .withArgs(thirdParty.address, 0)
          expect(notifierBalanceAfter.sub(notifierBalanceBefore)).to.equal(0)
        })
      })

      // TODO: Add a unit test ensuring `seize` call reverts if the staking
      // contract `seize` call reverts.
      // Currently blocked by https://github.com/defi-wonderland/smock/issues/101
      // See https://github.com/threshold-network/keep-core/issues/2870
    })
  })
})

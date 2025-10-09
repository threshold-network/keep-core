import { helpers } from "hardhat"
import { expect } from "chai"

import { walletRegistryFixture } from "./fixtures"
import ecdsaData from "./data/ecdsa"
import { createNewWallet } from "./utils/wallets"
import { signOperatorInactivityClaim } from "./utils/inactivity"

import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { FakeContract } from "@defi-wonderland/smock"
import type { Operator, OperatorID } from "./utils/operators"
import type {
  SortitionPool,
  WalletRegistry,
  WalletRegistryGovernance,
  TokenStaking,
  IWalletOwner,
  T,
  IRandomBeacon,
} from "../typechain"

const { to1e18 } = helpers.number

const { createSnapshot, restoreSnapshot } = helpers.snapshot

describe("WalletRegistry - Rewards", () => {
  let tToken: T
  let walletRegistry: WalletRegistry
  let staking: TokenStaking
  let walletRegistryGovernance: WalletRegistryGovernance
  let sortitionPool: SortitionPool
  let randomBeacon: FakeContract<IRandomBeacon>
  let walletOwner: FakeContract<IWalletOwner>

  let deployer: SignerWithAddress
  let governance: SignerWithAddress
  let thirdParty: SignerWithAddress

  let members: Operator[]
  let membersIDs: OperatorID[]
  let walletID: string

  const walletPublicKey: string = ecdsaData.group1.publicKey

  const rewardAmount = to1e18(100000)

  before("load test fixture", async () => {
    // eslint-disable-next-line @typescript-eslint/no-extra-semi
    ;({
      tToken,
      walletRegistry,
      walletRegistryGovernance,
      staking,
      sortitionPool,
      randomBeacon,
      walletOwner,
      deployer,
      governance,
      thirdParty,
    } = await walletRegistryFixture())
    ;({ members, walletID } = await createNewWallet(
      walletRegistry,
      walletOwner.wallet,
      randomBeacon,
      walletPublicKey
    ))

    membersIDs = members.map((member) => member.id)
  })

  describe("withdrawRewards", () => {
    context("when called for an unknown operator", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.withdrawRewards(thirdParty.address)
        ).to.be.revertedWith("UnknownOperator")
      })
    })

    context("when called for a known operator", () => {
      let stakingProvider: string
      let operator: string
      let beneficiary: string

      before(async () => {
        await createSnapshot()

        operator = members[0].signer.address
        stakingProvider = await walletRegistry.operatorToStakingProvider(
          operator
        )
        // eslint-disable-next-line @typescript-eslint/no-extra-semi
        ;({ beneficiary } = await staking.rolesOf(stakingProvider))

        // Allocate sortition pool rewards
        await tToken.connect(deployer).mint(deployer.address, rewardAmount)
        await tToken
          .connect(deployer)
          .approveAndCall(sortitionPool.address, rewardAmount, [])
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should withdraw rewards", async () => {
        expect(await tToken.balanceOf(beneficiary)).to.equal(0)
        await walletRegistry.withdrawRewards(stakingProvider)
        expect(await tToken.balanceOf(beneficiary)).to.be.gt(0)
      })

      it("should emit RewardsWithdrawn event", async () => {
        const balanceBefore = await tToken.balanceOf(beneficiary)
        const tx = await walletRegistry.withdrawRewards(stakingProvider)
        const balanceAfter = await tToken.balanceOf(beneficiary)
        const received = balanceAfter.sub(balanceBefore)

        await expect(tx)
          .to.emit(walletRegistry, "RewardsWithdrawn")
          .withArgs(stakingProvider, received)
      })
    })
  })

  describe("availableRewards", () => {
    context("when called for an unknown operator", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.availableRewards(thirdParty.address)
        ).to.be.revertedWith("UnknownOperator")
      })
    })

    context("when called for a known operator", () => {
      let stakingProvider: string
      let operator: string
      let beneficiary: string

      before(async () => {
        await createSnapshot()

        operator = members[0].signer.address
        stakingProvider = await walletRegistry.operatorToStakingProvider(
          operator
        )
        // eslint-disable-next-line @typescript-eslint/no-extra-semi
        ;({ beneficiary } = await staking.rolesOf(stakingProvider))

        // Allocate sortition pool rewards
        await tToken.connect(deployer).mint(deployer.address, rewardAmount)
        await tToken
          .connect(deployer)
          .approveAndCall(sortitionPool.address, rewardAmount, [])
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should return the amount of available rewards", async () => {
        let availableAmount = await walletRegistry.availableRewards(
          stakingProvider
        )

        const balanceBefore = await tToken.balanceOf(beneficiary)
        await walletRegistry.withdrawRewards(stakingProvider)
        const balanceAfter = await tToken.balanceOf(beneficiary)

        expect(availableAmount).to.equal(balanceAfter.sub(balanceBefore))

        availableAmount = await walletRegistry.availableRewards(stakingProvider)
        expect(availableAmount).to.equal(0)
      })
    })
  })

  describe("withdrawIneligibleRewards", () => {
    const inactiveMembersIndices = [1, 5, 10]
    const heartbeatFailed = false
    const groupThreshold = 51

    context("when called not by the governance", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(thirdParty)
            .withdrawIneligibleRewards(thirdParty.address)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by the governance", () => {
      before(async () => {
        await createSnapshot()

        // Assume claim sender is the first signing member.
        const claimSender = members[0].signer

        const { signatures, signingMembersIndices } =
          await signOperatorInactivityClaim(
            members,
            0,
            walletPublicKey,
            heartbeatFailed,
            inactiveMembersIndices,
            groupThreshold
          )

        await walletRegistry.connect(claimSender).notifyOperatorInactivity(
          {
            walletID,
            inactiveMembersIndices,
            heartbeatFailed,
            signatures,
            signingMembersIndices,
          },
          0,
          membersIDs
        )

        // Allocate sortition pool rewards
        await tToken.connect(deployer).mint(deployer.address, rewardAmount)
        await tToken
          .connect(deployer)
          .approveAndCall(sortitionPool.address, rewardAmount, [])
      })

      it("should withdraw ineligible rewards", async () => {
        // Withdraw rewards for ineligible operator. This action recalculates
        // the balance of "ineligible rewards" available for withdrawal from
        // the Sortition Pool
        const operator = members[0].signer.address
        const stakingProvider = await walletRegistry.operatorToStakingProvider(
          operator
        )
        await walletRegistry.withdrawRewards(stakingProvider)

        expect(await tToken.balanceOf(thirdParty.address)).to.equal(0)
        await walletRegistryGovernance
          .connect(governance)
          .withdrawIneligibleRewards(thirdParty.address)
        expect(await tToken.balanceOf(thirdParty.address)).to.be.gt(0)
      })
    })
  })
})

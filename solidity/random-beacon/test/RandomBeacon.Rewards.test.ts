import { helpers } from "hardhat"
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers"
import { expect } from "chai"

import { constants, randomBeaconDeployment } from "./fixtures"
import { registerOperators } from "./utils/operators"
import { createGroup } from "./utils/groups"
import { signOperatorInactivityClaim } from "./utils/inactivity"

import type { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers"
import type { Operator } from "./utils/operators"
import type {
  RandomBeacon,
  T,
  SortitionPool,
  TokenStaking,
  RandomBeaconGovernance,
} from "../typechain"

const { createSnapshot, restoreSnapshot } = helpers.snapshot
const { to1e18 } = helpers.number

const fixture = async () => {
  const contracts = await randomBeaconDeployment()

  // Accounts offset provided to slice getUnnamedSigners have to include number
  // of unnamed accounts that were already used.
  const operators = await registerOperators(
    contracts.randomBeacon as RandomBeacon,
    contracts.t as T,
    constants.groupSize,
    1,
  )

  const randomBeacon = contracts.randomBeacon as RandomBeacon
  const randomBeaconGovernance =
    contracts.randomBeaconGovernance as RandomBeaconGovernance
  const staking = contracts.staking as TokenStaking
  const sortitionPool = contracts.sortitionPool as SortitionPool
  const t = contracts.t as T

  return {
    randomBeacon,
    randomBeaconGovernance,
    staking,
    sortitionPool,
    t,
    operators,
  }
}

describe("RandomBeacon - Rewards", () => {
  let deployer: SignerWithAddress
  let governance: SignerWithAddress
  let thirdParty: SignerWithAddress
  let operators: Operator[]

  let randomBeacon: RandomBeacon
  let randomBeaconGovernance: RandomBeaconGovernance
  let staking: TokenStaking
  let sortitionPool: SortitionPool
  let t: T

  const rewardAmount = to1e18(100000)

  before(async () => {
    ;({
      randomBeacon,
      randomBeaconGovernance,
      staking,
      sortitionPool,
      t,
      operators,
    } = await loadFixture(fixture))
    ;[thirdParty] = await helpers.signers.getUnnamedSigners()
    ;({ deployer, governance } = await helpers.signers.getNamedSigners())
  })

  describe("withdrawRewards", () => {
    context("when called for an unknown operator", () => {
      it("should revert", async () => {
        await expect(
          randomBeacon.withdrawRewards(thirdParty.address),
        ).to.be.revertedWith("Unknown operator")
      })
    })

    context("when called for a known operator", () => {
      let stakingProvider: string
      let operator: string
      let beneficiary: string

      before(async () => {
        await createSnapshot()

        operator = operators[0].signer.address
        stakingProvider = await randomBeacon.operatorToStakingProvider(operator)
        ;({ beneficiary } = await staking.rolesOf(stakingProvider))

        // Allocate sortition pool rewards
        await t.connect(deployer).mint(deployer.address, rewardAmount)
        await t
          .connect(deployer)
          .approveAndCall(await sortitionPool.getAddress(), rewardAmount, "0x")
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should withdraw rewards", async () => {
        expect(await t.balanceOf(beneficiary)).to.equal(0)
        await randomBeacon.withdrawRewards(stakingProvider)
        expect(await t.balanceOf(beneficiary)).to.be.gt(0)
      })

      it("should emit RewardsWithdrawn event", async () => {
        const balanceBefore = await t.balanceOf(beneficiary)
        const tx = await randomBeacon.withdrawRewards(stakingProvider)
        const balanceAfter = await t.balanceOf(beneficiary)
        const received = balanceAfter - balanceBefore

        await expect(tx)
          .to.emit(randomBeacon, "RewardsWithdrawn")
          .withArgs(stakingProvider, received)
      })
    })
  })

  describe("availableRewards", () => {
    context("when called for an unknown operator", () => {
      it("should revert", async () => {
        await expect(
          randomBeacon.availableRewards(thirdParty.address),
        ).to.be.revertedWith("Unknown operator")
      })
    })

    context("when called for a known operator", () => {
      let stakingProvider: string
      let operator: string
      let beneficiary: string

      before(async () => {
        await createSnapshot()

        operator = operators[0].signer.address
        stakingProvider = await randomBeacon.operatorToStakingProvider(operator)
        ;({ beneficiary } = await staking.rolesOf(stakingProvider))

        // Allocate sortition pool rewards
        await t.connect(deployer).mint(deployer.address, rewardAmount)
        await t
          .connect(deployer)
          .approveAndCall(await sortitionPool.getAddress(), rewardAmount, "0x")
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should return the amount of available rewards", async () => {
        let availableAmount =
          await randomBeacon.availableRewards(stakingProvider)

        const balanceBefore = await t.balanceOf(beneficiary)
        await randomBeacon.withdrawRewards(stakingProvider)
        const balanceAfter = await t.balanceOf(beneficiary)

        expect(availableAmount).to.equal(balanceAfter - balanceBefore)

        availableAmount = await randomBeacon.availableRewards(stakingProvider)
        expect(availableAmount).to.equal(0)
      })
    })
  })

  describe("withdrawIneligibleRewards", () => {
    context("when called not by the governance", () => {
      it("should revert", async () => {
        await expect(
          randomBeacon
            .connect(thirdParty)
            .withdrawIneligibleRewards(thirdParty.address),
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by the governance", () => {
      before(async () => {
        await createSnapshot()

        await createGroup(randomBeacon, operators)
        const groupId = 0
        const group = await randomBeacon["getGroup(uint64)"](groupId)

        const inactiveMembersIndices = [1, 2, 3]

        const { signatures, signingMembersIndices } =
          await signOperatorInactivityClaim(
            operators,
            0,
            group.groupPubKey,
            inactiveMembersIndices,
            33,
          )

        const claimSender = operators[0].signer
        await randomBeacon.connect(claimSender).notifyOperatorInactivity(
          {
            groupId,
            inactiveMembersIndices,
            signatures,
            signingMembersIndices,
          },
          0,
          operators.map((operator) => operator.id),
        )

        // Allocate sortition pool rewards
        await t.connect(deployer).mint(deployer.address, rewardAmount)
        await t
          .connect(deployer)
          .approveAndCall(await sortitionPool.getAddress(), rewardAmount, "0x")
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should withdraw ineligible rewards", async () => {
        // Withdraw rewards for ineligible operator. This action recalculates
        // the balance of "ineligible rewards" available for withdrawal from
        // the Sortition Pool
        const operator = operators[0].signer.address
        const stakingProvider =
          await randomBeacon.operatorToStakingProvider(operator)
        await randomBeacon.withdrawRewards(stakingProvider)

        expect(await t.balanceOf(thirdParty.address)).to.equal(0)
        await randomBeaconGovernance
          .connect(governance)
          .withdrawIneligibleRewards(thirdParty.address)
        expect(await t.balanceOf(thirdParty.address)).to.be.gt(0)
      })
    })
  })
})

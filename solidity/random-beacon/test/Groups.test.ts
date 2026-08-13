import { ethers, waffle } from "hardhat"
import { expect } from "chai"

import blsData from "./data/bls"
import { constants } from "./fixtures"
import { noMisbehaved, hashDKGMembers } from "./utils/dkg"
import { hashUint32Array } from "./utils/groups"

import type { GroupsStub } from "../typechain"
import type { ContractTransaction } from "ethers"
import type { Groups } from "../typechain/contracts/test/GroupsStub"

const { keccak256 } = ethers.utils

const fixture = async () => {
  const GroupsStub = await ethers.getContractFactory("GroupsStub")
  const groups = await GroupsStub.deploy()

  return groups
}

describe("Groups", () => {
  const groupPublicKey: string = ethers.utils.hexValue(blsData.groupPubKey)
  const members: number[] = []

  let groups: GroupsStub

  before(async () => {
    for (let i = 0; i < constants.groupSize; i++) members.push(10000 + i)
  })

  beforeEach("load test fixture", async () => {
    groups = await waffle.loadFixture(fixture)
  })

  describe("validatePublicKey", async () => {
    beforeEach(async () => {
      await groups.addGroup(
        groupPublicKey,
        hashDKGMembers(members, noMisbehaved)
      )
    })

    context("when group is already registered", async () => {
      it("should revert with 'Group with this public key was already registered' error", async () => {
        await expect(
          groups.validatePublicKey(groupPublicKey)
        ).to.be.revertedWith(
          "Group with this public key was already registered"
        )
      })
    })
  })

  describe("addGroup", async () => {
    context("when no groups are registered", async () => {
      let tx: ContractTransaction

      context("with no misbehaved members", async () => {
        beforeEach(async () => {
          tx = await groups.addGroup(
            groupPublicKey,
            hashDKGMembers(members, noMisbehaved)
          )
        })

        it("should emit GroupRegistered event", async () => {
          await expect(tx)
            .to.emit(groups, "GroupRegistered")
            .withArgs(0, groupPublicKey)
        })

        it("should register group", async () => {
          const groupsRegistry = await groups.getGroupsRegistry()

          expect(groupsRegistry).to.be.lengthOf(1)
          expect(groupsRegistry[0]).to.deep.equal(keccak256(groupPublicKey))
        })

        it("should store group data", async () => {
          const storedGroup = await groups.getGroup(groupPublicKey)

          expect(storedGroup.groupPubKey).to.be.equal(groupPublicKey)
          expect(storedGroup.registrationBlockNumber).to.be.equal(
            tx.blockNumber
          )
          expect(storedGroup.membersHash).to.be.equal(hashUint32Array(members))
        })
      })

      context("with misbehaved members", async () => {
        context("with first member misbehaved", async () => {
          it("should filter out misbehaved members", async () => {
            const misbehavedIndices: number[] = [1]

            await groups.addGroup(
              groupPublicKey,
              hashDKGMembers(members, misbehavedIndices)
            )

            const expectedMembers = [...members]
            expectedMembers.splice(0, 1)
            const expectedMembersHash = hashUint32Array(expectedMembers)

            expect(
              (await groups.getGroup(groupPublicKey)).membersHash
            ).to.be.equal(expectedMembersHash)
          })
        })

        context("with last member misbehaved", async () => {
          it("should filter out misbehaved members", async () => {
            const misbehavedIndices: number[] = [constants.groupSize]
            await groups.addGroup(
              groupPublicKey,
              hashDKGMembers(members, misbehavedIndices)
            )
            const expectedMembers = [...members]
            expectedMembers.pop()
            const expectedMembersHash = hashUint32Array(expectedMembers)

            expect(
              (await groups.getGroup(groupPublicKey)).membersHash
            ).to.be.equal(expectedMembersHash)
          })
        })

        context("with middle member misbehaved", async () => {
          it("should filter out misbehaved members", async () => {
            const misbehavedIndices: number[] = [24]
            await groups.addGroup(
              groupPublicKey,
              hashDKGMembers(members, misbehavedIndices)
            )

            const expectedMembers = [...members]
            expectedMembers.splice(23, 1)
            const expectedMembersHash = hashUint32Array(expectedMembers)

            expect(
              (await groups.getGroup(groupPublicKey)).membersHash
            ).to.be.equal(expectedMembersHash)
          })
        })

        context("with multiple members misbehaved", async () => {
          it("should filter out misbehaved members", async () => {
            const misbehavedIndices: number[] = [1, 16, 35, constants.groupSize]
            await groups.addGroup(
              groupPublicKey,
              hashDKGMembers(members, misbehavedIndices)
            )
            const expectedMembers = [...members]
            expectedMembers.splice(0, 1) // index -1
            expectedMembers.splice(14, 1) // index -2 (cause expectedMembers already shrinked)
            expectedMembers.splice(32, 1) // index -3
            expectedMembers.splice(constants.groupSize - 4, 1) // index -4

            const expectedMembersHash = hashUint32Array(expectedMembers)

            expect(
              (await groups.getGroup(groupPublicKey)).membersHash
            ).to.be.equal(expectedMembersHash)
          })
        })
      })
    })

    context("when existing group is already registered", async () => {
      const existingGroupPublicKey = "0x1234567890"

      let existingGroupMembers: number[]
      let newGroupMembers: number[]
      let existingGroup: Groups.GroupStructOutput

      beforeEach(async () => {
        existingGroupMembers = members.slice(30)
        newGroupMembers = members.slice(-30)

        await groups.addGroup(
          existingGroupPublicKey,
          hashDKGMembers(existingGroupMembers, noMisbehaved)
        )

        existingGroup = await groups.getGroup(existingGroupPublicKey)
      })

      context("with unique group public key", async () => {
        const newGroupPublicKey = groupPublicKey

        let tx: ContractTransaction

        beforeEach(async () => {
          tx = await groups.addGroup(
            newGroupPublicKey,
            hashDKGMembers(newGroupMembers, noMisbehaved)
          )
        })

        it("should emit GroupRegistered event", async () => {
          await expect(tx)
            .to.emit(groups, "GroupRegistered")
            .withArgs(1, newGroupPublicKey)
        })

        it("should register group", async () => {
          const groupsRegistry = await groups.getGroupsRegistry()

          expect(groupsRegistry).to.be.lengthOf(2)
          expect(groupsRegistry[1]).to.deep.equal(keccak256(newGroupPublicKey))
        })

        it("should store group data", async () => {
          const storedGroup = await groups.getGroup(newGroupPublicKey)

          expect(storedGroup.groupPubKey).to.be.equal(newGroupPublicKey)
          expect(storedGroup.registrationBlockNumber).to.be.equal(
            tx.blockNumber
          )
          expect(storedGroup.membersHash).to.be.equal(
            hashUint32Array(newGroupMembers)
          )
        })

        it("should not update existing group", async () => {
          const groupsRegistry = await groups.getGroupsRegistry()

          expect(groupsRegistry[0]).to.deep.equal(
            keccak256(existingGroupPublicKey)
          )

          const storedGroup = await groups.getGroup(existingGroupPublicKey)

          expect(storedGroup.groupPubKey).to.be.equal(existingGroupPublicKey)
          expect(storedGroup.registrationBlockNumber).to.be.equal(
            existingGroup.registrationBlockNumber
          )

          expect(storedGroup.membersHash).to.be.equal(
            hashUint32Array(existingGroupMembers)
          )
        })
      })
    })
  })
})

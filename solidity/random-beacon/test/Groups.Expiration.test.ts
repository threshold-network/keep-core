import { ethers, helpers } from "hardhat"
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers"
import { expect } from "chai"

import { noMisbehaved, hashDKGMembers } from "./utils/dkg"

import type { BigNumberish } from "ethers"
import type { GroupsStub } from "../typechain"

const fixture = async () => {
  const GroupsStub = await ethers.getContractFactory("GroupsStub")
  const groups = await GroupsStub.deploy()

  return groups
}

const { mineBlocksTo } = helpers.time

describe("Groups", () => {
  const members: number[] = []
  const groupLifetime = 20

  let groups: GroupsStub

  beforeEach("load test fixture", async () => {
    groups = await loadFixture(fixture)
  })

  describe("expireOldGroups", async () => {
    beforeEach(async () => {
      await groups.setGroupLifetime(groupLifetime)
    })

    context("when active groups were created", async () => {
      it("should be able to count the number of active groups", async () => {
        const expectedGroupCount = 23
        await addGroups(1, expectedGroupCount)
        const numberOfGroups = await groups.numberOfActiveGroups()
        expect(numberOfGroups).to.be.equal(expectedGroupCount)
      })
    })

    context("when expiring old groups and selecting active ones", async () => {
      it("A beacon_value = 0", async () => {
        const selectedIndex = await runExpirationTest(1, 0, 0)
        expect(selectedIndex).to.be.equal(0)
      })
      it("A beacon_value = 1", async () => {
        const selectedIndex = await runExpirationTest(1, 0, 1)
        expect(selectedIndex).to.be.equal(0)
      })
      it("AAA beacon_value = 0", async () => {
        const selectedIndex = await runExpirationTest(3, 0, 0)
        expect(selectedIndex).to.be.equal(0)
      })
      it("AAA beacon_value = 1", async () => {
        const selectedIndex = await runExpirationTest(3, 0, 1)
        expect(selectedIndex).to.be.equal(1)
      })
      it("AAA beacon_value = 2", async () => {
        const selectedIndex = await runExpirationTest(3, 0, 2)
        expect(selectedIndex).to.be.equal(2)
      })
      it("AAA beacon_value = 3", async () => {
        const selectedIndex = await runExpirationTest(3, 0, 3)
        expect(selectedIndex).to.be.equal(0)
      })
      it("EAA beacon_value = 0", async () => {
        const selectedIndex = await runExpirationTest(3, 1, 0)
        expect(selectedIndex).to.be.equal(1)
      })
      it("EEEEAAAAAA beacon_value = 0", async () => {
        const selectedIndex = await runExpirationTest(10, 4, 0)
        expect(selectedIndex).to.be.equal(4)
      })
      it("EEEEAAAAAA beacon_value = 1", async () => {
        const selectedIndex = await runExpirationTest(10, 4, 1)
        expect(selectedIndex).to.be.equal(5)
      })
      it("EEEEAAAAAA beacon_value = 2", async () => {
        const selectedIndex = await runExpirationTest(10, 4, 2)
        expect(selectedIndex).to.be.equal(6)
      })
      it("EEEEAAAAAA beacon_value = 3", async () => {
        const selectedIndex = await runExpirationTest(10, 4, 3)
        expect(selectedIndex).to.be.equal(7)
      })
      it("EEEEAAAAAA beacon_value = 4", async () => {
        const selectedIndex = await runExpirationTest(10, 4, 4)
        expect(selectedIndex).to.be.equal(8)
      })
      it("EEEEAAAAAA beacon_value = 5", async () => {
        const selectedIndex = await runExpirationTest(10, 4, 5)
        expect(selectedIndex).to.be.equal(9)
      })
      it("EEEEAAAAAA beacon_value = 6", async () => {
        const selectedIndex = await runExpirationTest(10, 4, 6)
        expect(selectedIndex).to.be.equal(4)
      })
      it("EEEEAAAAAA beacon_value = 7", async () => {
        const selectedIndex = await runExpirationTest(10, 4, 7)
        expect(selectedIndex).to.be.equal(5)
      })
      it("EEEEAAAAAA beacon_value = 8", async () => {
        const selectedIndex = await runExpirationTest(10, 4, 8)
        expect(selectedIndex).to.be.equal(6)
      })
      it("EEEEAAAAAA beacon_value = 9", async () => {
        const selectedIndex = await runExpirationTest(10, 4, 9)
        expect(selectedIndex).to.be.equal(7)
      })
      it("EEEEAAAAAA beacon_value = 10", async () => {
        const selectedIndex = await runExpirationTest(10, 4, 10)
        expect(selectedIndex).to.be.equal(8)
      })
      it("EEEEAAAAAA beacon_value = 11", async () => {
        const selectedIndex = await runExpirationTest(10, 4, 11)
        expect(selectedIndex).to.be.equal(9)
      })
      it("EEEEEEEEEA beacon_value = 0", async () => {
        const selectedIndex = await runExpirationTest(10, 9, 0)
        expect(selectedIndex).to.be.equal(9)
      })
      it("EEEEEEEEEA beacon_value = 1", async () => {
        const selectedIndex = await runExpirationTest(10, 9, 1)
        expect(selectedIndex).to.be.equal(9)
      })
      it("EEEEEEEEEA beacon_value = 10", async () => {
        const selectedIndex = await runExpirationTest(10, 9, 10)
        expect(selectedIndex).to.be.equal(9)
      })
      it("EEEEEEEEEA beacon_value = 11", async () => {
        const selectedIndex = await runExpirationTest(10, 9, 11)
        expect(selectedIndex).to.be.equal(9)
      })
    })

    context("when all groups expired", async () => {
      it("should revert group selection", async () => {
        await addGroups(1, 5)

        const currentBlock = await ethers.provider.getBlock("latest")
        await mineBlocksTo(currentBlock.number + groupLifetime)

        await expect(groups.selectGroup(0)).to.be.revertedWith(
          "No active groups",
        )
      })

      it("should allow to add and select new group", async () => {
        await addGroups(1, 5)
        const currentBlock = await ethers.provider.getBlock("latest")
        await mineBlocksTo(currentBlock.number + groupLifetime)

        await groups.addGroup(
          ethers.utils.hexlify(6),
          hashDKGMembers(members, noMisbehaved),
        )

        const selected = await groups.callStatic.selectGroup(0)
        await groups.selectGroup(0)
        const numberOfGroups = await groups.numberOfActiveGroups()

        expect(numberOfGroups).to.be.equal(1)
        expect(selected).to.be.equal(5)
      })
    })

    context("when having a mix of terminated and expired groups", async () => {
      it("EEETTAAAAA beacon_value seed = 5", async () => {
        // existing: []
        // new: [0x1,0x2,0x3]
        await addGroups(1, 3)
        await expireGroup(2) // expiring [0x1,0x2,0x3]

        await groups.expireOldGroups()

        // existing: [0x1, 0x2, 0x3]
        // new: [0x4, 0x5]
        await addGroups(4, 2)
        await addTerminatedGroups(3, 2) // terminating [0x4, 0x5]

        // move blocks so terminated blocks qualify for expiration
        const currentBlock = await ethers.provider.getBlock("latest")
        await mineBlocksTo(currentBlock.number + groupLifetime)

        // [0x1, 0x2, 0x3, 0x4, 0x5]
        // new: [0x6, 0x7, 0x8, 0x9, 0xa]
        await addGroups(6, 5)

        // First active index group that qualifies for selection
        const selectedGroupId = await groups.callStatic.selectGroup(5)
        expect(selectedGroupId).to.be.equal(5)
      })

      it("ETTAAAAAAA beacon_value seed = 1", async () => {
        await addGroups(1, 1) // [0x1]
        await expireGroup(0) // expiring [0x1]

        await addGroups(2, 2) // [0x1] + [0x2,0x3]
        await addTerminatedGroups(1, 2) // terminating [0x2,0x3]

        await addGroups(4, 7) // [0x4,0x5,0x6,0x7,0x8,0x9,0xa]

        // First active index group that qualifies for selection
        const selectedGroupId = await groups.callStatic.selectGroup(1)
        // 1 expired + 2 terminated + selected index (1)
        expect(selectedGroupId).to.be.equal(4)
      })

      it("ETEATATAAA beacon_value seed = 2", async () => {
        await addGroups(1, 3) // [0x1,0x2,0x3]
        await groups.terminateGroup(1) // [0x2]

        // move blocks so terminated blocks qualify for expiration
        const currentBlock = await ethers.provider.getBlock("latest")
        await mineBlocksTo(currentBlock.number + groupLifetime)

        let activeTerminatedGroups = await groups.activeTerminatedGroups()
        expect(activeTerminatedGroups.length).to.be.equal(1)

        // 10 groups were created in total
        await addGroups(4, 7) // [0x4,...,0xa]

        await groups.terminateGroup(4)
        await groups.terminateGroup(6)

        await groups.expireOldGroups()

        // two terminated groups do not qualify yet to be expired because of the
        // current block #
        activeTerminatedGroups = await groups.activeTerminatedGroups()
        expect(activeTerminatedGroups.length).to.be.equal(2)
        expect(activeTerminatedGroups[0]).to.be.equal(4)
        expect(activeTerminatedGroups[1]).to.be.equal(6)

        // 10 - 3 (expired) - 2 (terminated) = 5
        const numberOfGroups = await groups.numberOfActiveGroups()
        expect(numberOfGroups).to.be.equal(5)

        // Second active index group that qualifies for selection
        const selectedIndex = await groups.callStatic.selectGroup(2)

        // expired ids: [0, 1, 2]
        // terminated ids: [4, 6]
        // active ids: [3, 5, 7, 8, 9]
        expect(7).to.be.equal(selectedIndex)
      })

      it("EEEEEEEEET beacon_value seed = 2", async () => {
        await addGroups(1, 9) // [0x1,..0x9]
        await expireGroup(8) // expiring [0x1,..0x9]

        await addGroups(10, 1) // [0x1,..0x9] + [0xa]
        await groups.terminateGroup(9) // terminating [0xa]

        await expect(groups.selectGroup(2)).to.be.revertedWith(
          "No active groups",
        )
      })

      it("should expire all active terminated groups when all of them qualify for expiration", async () => {
        await addGroups(1, 31)

        await groups.terminateGroup(10)
        await groups.terminateGroup(12)
        await groups.terminateGroup(20)
        await groups.terminateGroup(25)
        await groups.terminateGroup(30)

        // move blocks so terminated blocks qualify for expiration
        const currentBlock = await ethers.provider.getBlock("latest")
        await mineBlocksTo(currentBlock.number + groupLifetime)

        await groups.expireOldGroups()

        const activeTerminatedGroups = await groups.activeTerminatedGroups()
        expect(activeTerminatedGroups.length).to.be.equal(0)

        // Total number of groups (expired + active) is equal to 100 now
        await addGroups(32, 69)

        const numberOfGroups = await groups.numberOfActiveGroups()
        expect(numberOfGroups).to.be.equal(69)
        const expiredGroupOffset = await groups.expiredGroupOffset()
        expect(expiredGroupOffset).to.be.equal(31)
      })
    })
  })

  async function addGroups(firstGroup: number, numberOfGroups: number) {
    for (let i = firstGroup; i < firstGroup + numberOfGroups; i++) {
      await groups.addGroup(
        ethers.utils.hexlify(i),
        hashDKGMembers(members, noMisbehaved),
      )
    }
  }

  async function expireGroup(groupId: BigNumberish) {
    const group = await groups.getGroupById(groupId)
    const registrationBlock = group.registrationBlockNumber
    const currentBlock = await ethers.provider.getBlock("latest")

    if (currentBlock.number - registrationBlock.toNumber() <= groupLifetime) {
      const minedBlocksToExpireGroup =
        currentBlock.number +
        (groupLifetime - (currentBlock.number - registrationBlock.toNumber())) +
        1
      await mineBlocksTo(minedBlocksToExpireGroup)
    }
  }

  async function addTerminatedGroups(
    firstGroupIdToTerminate: number,
    numberOfTerminatedGroups: number,
  ) {
    for (
      let i = firstGroupIdToTerminate;
      i < firstGroupIdToTerminate + numberOfTerminatedGroups;
      i++
    ) {
      await groups.terminateGroup(i) // terminating by the group id
    }
  }

  async function runExpirationTest(
    numberOfGroups: number,
    expiredCount: number,
    beaconValue: BigNumberish,
  ) {
    await addGroups(1, numberOfGroups)
    if (expiredCount > 0) {
      // expire group accepts group index, we need to subtract one from the
      // count since we index from 0.
      await expireGroup(expiredCount - 1)
    }
    return groups.callStatic.selectGroup(beaconValue)
  }
})

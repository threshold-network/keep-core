import { ethers, helpers } from "hardhat"
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers"
import { expect } from "chai"

import requireResult from "./helpers/chain"
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

  describe("terminateGroup", async () => {
    beforeEach(async () => {
      await groups.setGroupLifetime(groupLifetime)
    })

    context(
      "when performing selection with terminated groups in non ascending order",
      async () => {
        it("AATAT beacon_value = 0", async () => {
          const selectedIndex = await runTerminationTest(5, 0, [4, 2], 0)
          expect(selectedIndex).to.be.equal(0)
        })
        it("AATAT beacon_value = 1", async () => {
          const selectedIndex = await runTerminationTest(5, 0, [4, 2], 1)
          expect(1).to.be.equal(selectedIndex)
        })
        it("AATAT beacon_value = 2", async () => {
          const selectedIndex = await runTerminationTest(5, 0, [4, 2], 2)
          expect(3).to.be.equal(selectedIndex)
        })
        it("TATATTA beacon_value = 0", async () => {
          const selectedIndex = await runTerminationTest(7, 0, [5, 2, 4, 0], 0)
          expect(1).to.be.equal(selectedIndex)
        })
        it("TATATTA beacon_value = 1", async () => {
          const selectedIndex = await runTerminationTest(7, 0, [5, 2, 4, 0], 1)
          expect(3).to.be.equal(selectedIndex)
        })
        it("TATATTA beacon_value = 2", async () => {
          const selectedIndex = await runTerminationTest(7, 0, [5, 2, 4, 0], 2)
          expect(6).to.be.equal(selectedIndex)
        })
        it("AATATTA beacon_value = 0", async () => {
          const selectedIndex = await runTerminationTest(7, 0, [5, 2, 4], 0)
          expect(0).to.be.equal(selectedIndex)
        })
        it("TATATAAT beacon_value = 0", async () => {
          const selectedIndex = await runTerminationTest(8, 0, [7, 0, 4, 2], 0)
          expect(1).to.be.equal(selectedIndex)
        })
        it("TATATAAT beacon_value = 1", async () => {
          const selectedIndex = await runTerminationTest(8, 0, [7, 0, 4, 2], 1)
          expect(3).to.be.equal(selectedIndex)
        })
        it("TATATAAT beacon_value = 2", async () => {
          const selectedIndex = await runTerminationTest(8, 0, [7, 0, 4, 2], 2)
          expect(5).to.be.equal(selectedIndex)
        })
        it("TATATAAT beacon_value = 3", async () => {
          const selectedIndex = await runTerminationTest(8, 0, [7, 0, 4, 2], 3)
          expect(6).to.be.equal(selectedIndex)
        })
      },
    )

    context("when not selecting terminated groups", async () => {
      it("TA beacon_value = 0", async () => {
        const selectedIndex = await runTerminationTest(2, 0, [0], 0)
        expect(1).to.be.equal(selectedIndex)
      })
      it("TA beacon_value = 1", async () => {
        const selectedIndex = await runTerminationTest(2, 0, [0], 1)
        expect(1).to.be.equal(selectedIndex)
      })
      it("TA beacon_value = 2", async () => {
        const selectedIndex = await runTerminationTest(2, 0, [0], 2)
        expect(1).to.be.equal(selectedIndex)
      })
      it("AT beacon_value = 0", async () => {
        const selectedIndex = await runTerminationTest(2, 0, [1], 0)
        expect(0).to.be.equal(selectedIndex)
      })
      it("AT beacon_value = 1", async () => {
        const selectedIndex = await runTerminationTest(2, 0, [1], 1)
        expect(0).to.be.equal(selectedIndex)
      })
      it("AT beacon_value = 2", async () => {
        const selectedIndex = await runTerminationTest(2, 0, [1], 2)
        expect(0).to.be.equal(selectedIndex)
      })
      it("TAA beacon_value = 0", async () => {
        const selectedIndex = await runTerminationTest(3, 0, [0], 0)
        expect(1).to.be.equal(selectedIndex)
      })
      it("TAA beacon_value = 1", async () => {
        const selectedIndex = await runTerminationTest(3, 0, [0], 1)
        expect(2).to.be.equal(selectedIndex)
      })
      it("TAA beacon_value = 2", async () => {
        const selectedIndex = await runTerminationTest(3, 0, [0], 2)
        expect(1).to.be.equal(selectedIndex)
      })
      it("AAT beacon_value = 0", async () => {
        const selectedIndex = await runTerminationTest(3, 0, [2], 0)
        expect(0).to.be.equal(selectedIndex)
      })
      it("AAT beacon_value = 1", async () => {
        const selectedIndex = await runTerminationTest(3, 0, [2], 1)
        expect(1).to.be.equal(selectedIndex)
      })
      it("AAT beacon_value = 2", async () => {
        const selectedIndex = await runTerminationTest(3, 0, [2], 2)
        expect(0).to.be.equal(selectedIndex)
      })
      it("ATA beacon_value = 0", async () => {
        const selectedIndex = await runTerminationTest(3, 0, [1], 0)
        expect(0).to.be.equal(selectedIndex)
      })
      it("ATA beacon_value = 1", async () => {
        const selectedIndex = await runTerminationTest(3, 0, [1], 1)
        expect(2).to.be.equal(selectedIndex)
      })
      it("ATA beacon_value = 2", async () => {
        const selectedIndex = await runTerminationTest(3, 0, [1], 2)
        expect(0).to.be.equal(selectedIndex)
      })
      it("TTA beacon_value = 0", async () => {
        const selectedIndex = await runTerminationTest(3, 0, [0, 1], 0)
        expect(2).to.be.equal(selectedIndex)
      })
      it("TTA beacon_value = 1", async () => {
        const selectedIndex = await runTerminationTest(3, 0, [0, 1], 1)
        expect(2).to.be.equal(selectedIndex)
      })
      it("ATT beacon_value = 0", async () => {
        const selectedIndex = await runTerminationTest(3, 0, [1, 2], 0)
        expect(0).to.be.equal(selectedIndex)
      })
      it("ATT beacon_value = 1", async () => {
        const selectedIndex = await runTerminationTest(3, 0, [1, 2], 1)
        expect(0).to.be.equal(selectedIndex)
      })
    })

    context(
      "when selecting neither expired nor terminated groups",
      async () => {
        it("ETA beacon_value = 0", async () => {
          const selectedIndex = await runTerminationTest(3, 1, [1], 0)
          expect(2).to.be.equal(selectedIndex)
        })
        it("ETA beacon_value = 1", async () => {
          const selectedIndex = await runTerminationTest(3, 1, [1], 1)
          expect(2).to.be.equal(selectedIndex)
        })
        it("ETA beacon_value = 2", async () => {
          const selectedIndex = await runTerminationTest(3, 1, [1], 2)
          expect(2).to.be.equal(selectedIndex)
        })
        it("ETA beacon_value = 3", async () => {
          const selectedIndex = await runTerminationTest(3, 1, [1], 3)
          expect(2).to.be.equal(selectedIndex)
        })
        it("EAT beacon_value = 0", async () => {
          const selectedIndex = await runTerminationTest(3, 1, [2], 0)
          expect(1).to.be.equal(selectedIndex)
        })
        it("EAT beacon_value = 1", async () => {
          const selectedIndex = await runTerminationTest(3, 1, [2], 1)
          expect(1).to.be.equal(selectedIndex)
        })
        it("EAT beacon_value = 2", async () => {
          const selectedIndex = await runTerminationTest(3, 1, [2], 2)
          expect(1).to.be.equal(selectedIndex)
        })
        it("EAT beacon_value = 3", async () => {
          const selectedIndex = await runTerminationTest(3, 1, [2], 3)
          expect(1).to.be.equal(selectedIndex)
        })
        it("EEETTATAAT beacon_value = 0", async () => {
          const selectedIndex = await runTerminationTest(10, 3, [3, 4, 6, 9], 0)
          expect(5).to.be.equal(selectedIndex)
        })
        it("EEETTATAAT beacon_value = 1", async () => {
          const selectedIndex = await runTerminationTest(10, 3, [3, 4, 6, 9], 1)
          expect(7).to.be.equal(selectedIndex)
        })
        it("EEETTATAAT beacon_value = 2", async () => {
          const selectedIndex = await runTerminationTest(10, 3, [3, 4, 6, 9], 2)
          expect(8).to.be.equal(selectedIndex)
        })
        it("EEETTATAAT beacon_value = 3", async () => {
          const selectedIndex = await runTerminationTest(10, 3, [3, 4, 6, 9], 3)
          expect(5).to.be.equal(selectedIndex)
        })
        it("EEETTATAAT beacon_value = 4", async () => {
          const selectedIndex = await runTerminationTest(10, 3, [3, 4, 6, 9], 4)
          expect(7).to.be.equal(selectedIndex)
        })
        it("EEETTATAAT beacon_value = 5", async () => {
          const selectedIndex = await runTerminationTest(10, 3, [3, 4, 6, 9], 5)
          expect(8).to.be.equal(selectedIndex)
        })
      },
    )

    context("when there are no active groups", async () => {
      it("T", async () => {
        await expect(runTerminationTest(1, 0, [0], 0)).to.be.revertedWith(
          "No active groups",
        )
      })
      it("TT", async () => {
        await expect(runTerminationTest(2, 0, [0, 1], 0)).to.be.revertedWith(
          "No active groups",
        )
      })
      it("ET", async () => {
        await expect(runTerminationTest(2, 1, [1], 0)).to.be.revertedWith(
          "No active groups",
        )
      })
    })

    async function addGroups(start: number, numberOfGroups: number) {
      for (let i = start; i <= numberOfGroups; i++) {
        await groups.addGroup(
          ethers.toBeHex(i),
          hashDKGMembers(members, noMisbehaved),
        )
      }
    }

    async function runTerminationTest(
      groupsCount: number,
      expiredCount: number,
      terminatedGroups: number[],
      beaconValue: BigNumberish,
    ) {
      await addGroups(1, expiredCount)

      const currentBlock = requireResult(
        await ethers.provider.getBlock("latest"),
      )
      await mineBlocksTo(currentBlock.number + groupLifetime)

      await addGroups(expiredCount + 1, groupsCount)

      for (let i = 0; i < terminatedGroups.length; i++) {
        await groups.terminateGroup(terminatedGroups[i])
      }

      return groups.selectGroup.staticCall(beaconValue)
    }
  })
})

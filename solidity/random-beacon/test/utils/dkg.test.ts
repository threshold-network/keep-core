import { ethers } from "hardhat"
import { expect } from "chai"

import { hashDKGMembers } from "./dkg"

describe("hashDKGMembers", () => {
  const members: number[] = [100, 101, 102, 103, 104, 105, 106, 107, 108, 109]

  context("when there are no misbehaved members", () => {
    it("should hash all the members", () => {
      const misbehavedMembers: number[] = []
      const expectedMembers = members

      const actualHash = hashDKGMembers(members, misbehavedMembers)
      const expectedHash = ethers.utils.keccak256(
        ethers.utils.defaultAbiCoder.encode(["uint32[]"], [expectedMembers]),
      )

      expect(expectedHash).to.be.equal(actualHash)
    })
  })

  context("when the first member id is a misbehaved one", () => {
    it("should hash all but the first member", () => {
      // misbehaved member count starts from 1, not 0
      const misbehavedMembers = [1]
      // expectedMembers[misbehavedMembers - 1], hence removing "0"
      const expectedMembers = [101, 102, 103, 104, 105, 106, 107, 108, 109]

      const actualHash = hashDKGMembers(members, misbehavedMembers)
      const expectedHash = ethers.utils.keccak256(
        ethers.utils.defaultAbiCoder.encode(["uint32[]"], [expectedMembers]),
      )

      expect(expectedHash).to.be.equal(actualHash)
    })
  })

  context("when the last member id is a misbehaved", () => {
    it("should hash all but the last member", () => {
      // misbehaved member count starts from 1, not 0
      const misbehavedMembers = [10]
      // expectedMembers[misbehavedMembers - 1], hence removing "9"
      const expectedMembers = [100, 101, 102, 103, 104, 105, 106, 107, 108]

      const actualHash = hashDKGMembers(members, misbehavedMembers)
      const expectedHash = ethers.utils.keccak256(
        ethers.utils.defaultAbiCoder.encode(["uint32[]"], [expectedMembers]),
      )

      expect(expectedHash).to.be.equal(actualHash)
    })
  })

  context("when there are multiple misbehaved members", () => {
    it("should hash all but the mishbehaved members", () => {
      // misbehaved member count starts from 1, not 0
      const misbehavedMembers = [2, 6, 8]
      // expectedMembers[misbehavedMembers - 1], hence removing "101,105,107"
      const expectedMembers = [100, 102, 103, 104, 106, 108, 109]

      const actualHash = hashDKGMembers(members, misbehavedMembers)
      const expectedHash = ethers.utils.keccak256(
        ethers.utils.defaultAbiCoder.encode(["uint32[]"], [expectedMembers]),
      )

      expect(expectedHash).to.be.equal(actualHash)
    })
  })

  context(
    "when a misbehaved member is not present in the members array",
    () => {
      it("should hash all the members", () => {
        const misbehavedMembers = [0, 42]
        const expectedMembers = members

        const actualHash = hashDKGMembers(members, misbehavedMembers)
        const expectedHash = ethers.utils.keccak256(
          ethers.utils.defaultAbiCoder.encode(["uint32[]"], [expectedMembers]),
        )

        expect(expectedHash).to.be.equal(actualHash)
      })
    },
  )
})

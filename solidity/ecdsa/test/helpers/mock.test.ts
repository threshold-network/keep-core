import { ethers } from "hardhat"
import { expect } from "chai"

import { createMock, expectCalledWith } from "./mock"

import type { Mock } from "./mock"
import type { IMockTarget, MockTargetConsumer } from "../../typechain"

describe("MockContract", () => {
  let target: Mock<IMockTarget>
  let consumer: MockTargetConsumer

  beforeEach(async () => {
    target = await createMock<IMockTarget>("IMockTarget")

    const factory = await ethers.getContractFactory("MockTargetConsumer")
    consumer = (await factory.deploy(target.address)) as MockTargetConsumer
    await consumer.deployed()
  })

  describe("view functions reached by STATICCALL", () => {
    // The mock records calls, which is a storage write, and a storage write is
    // impossible under STATICCALL. If recording were unconditional every
    // stubbed view function would revert, so this is the case that decides
    // whether a contract-backed mock is viable at all.

    it("answers a stubbed view function", async () => {
      await target.readValue.returns(42)

      expect(await consumer.readValueThroughStaticCall(7)).to.equal(42)
    })

    it("answers a stubbed view function from a state-changing caller", async () => {
      await target.readValue.returns(99)

      await consumer.cacheValue(7)

      expect(await consumer.lastValue()).to.equal(99)
    })

    it("answers with a struct", async () => {
      const owner = ethers.Wallet.createRandom().address
      await target.readInfo.returns({
        owner,
        createdAt: 1234,
        active: true,
      })

      const info = await consumer.readInfoThroughStaticCall(1)

      expect(info.owner).to.equal(owner)
      expect(info.createdAt).to.equal(1234)
      expect(info.active).to.equal(true)
    })

    it("answers with multiple values", async () => {
      await target.readPair.returns([11, 22])

      const [first, second] = await consumer.readPairThroughStaticCall(1)

      expect(first).to.equal(11)
      expect(second).to.equal(22)
    })

    it("returns the zero value when unstubbed", async () => {
      // smock answers an unconfigured function with the zero value of its
      // return type rather than reverting. Empty returndata would not do:
      // Solidity checks returndatasize against the size its ABI expects and
      // reverts the caller on a short answer, so the helper installs a
      // correctly encoded zero for every function up front.
      expect(await consumer.readValueThroughStaticCall(7)).to.equal(0)
    })
  })

  describe("whenCalledWith", () => {
    it("takes precedence over the selector-wide default", async () => {
      await target.readValue.returns(1)
      await target.readValue.whenCalledWith(7).returns(700)

      expect(await consumer.readValueThroughStaticCall(7)).to.equal(700)
      expect(await consumer.readValueThroughStaticCall(8)).to.equal(1)
    })

    it("falls through to the default for non-matching arguments", async () => {
      await target.readValue.whenCalledWith(7).returns(700)

      expect(await consumer.readValueThroughStaticCall(8)).to.equal(0)
    })
  })

  describe("reverts", () => {
    it("reverts every call to the function", async () => {
      await target.doThing.reverts("nope")

      await expect(
        consumer.doThing(ethers.constants.AddressZero, 1)
      ).to.be.revertedWith("nope")
    })

    it("reverts only the matching arguments", async () => {
      const who = ethers.Wallet.createRandom().address
      await target.doThing.returns(true)
      await target.doThing.whenCalledWith(who, 5).reverts("blocked")

      await expect(consumer.doThing(who, 5)).to.be.revertedWith("blocked")
      await expect(consumer.doThing(who, 6)).to.not.be.reverted
    })
  })

  describe("call recording", () => {
    it("records arguments of a state-changing call", async () => {
      const who = ethers.Wallet.createRandom().address
      await target.doThing.returns(true)

      await consumer.doThing(who, 123)

      expect(await target.doThing.callCount()).to.equal(1)

      const call = await target.doThing.getCall(0)
      expect(call.args[0]).to.equal(who)
      expect(call.args[1]).to.equal(123)
    })

    it("expectCalledWith matches any recorded call, not only a lone one", async () => {
      await consumer.noReturn(5)
      await consumer.noReturn(6)

      // `calledWith` says nothing about how many calls there were, so both
      // recorded calls must satisfy it and a third value must not.
      await expectCalledWith(target.noReturn, [5])
      await expectCalledWith(target.noReturn, [6])

      let failed = false
      try {
        await expectCalledWith(target.noReturn, [7])
      } catch {
        failed = true
      }
      expect(failed, "expected a non-matching argument to fail").to.equal(true)
    })

    it("records a function that returns nothing", async () => {
      await consumer.noReturn(5)
      await consumer.noReturn(6)

      expect(await target.noReturn.callCount()).to.equal(2)
      expect((await target.noReturn.getCall(1)).args[0]).to.equal(6)
    })

    it("counts each function separately", async () => {
      await target.doThing.returns(true)

      await consumer.doThing(ethers.constants.AddressZero, 1)
      await consumer.noReturn(1)

      expect(await target.doThing.callCount()).to.equal(1)
      expect(await target.noReturn.callCount()).to.equal(1)
    })
  })

  describe("reset", () => {
    it("clears recorded calls and configured responses for one function", async () => {
      await target.doThing.returns(true)
      await consumer.doThing(ethers.constants.AddressZero, 1)

      await target.doThing.reset()

      expect(await target.doThing.callCount()).to.equal(0)
      // The configured `true` is gone, so the call answers with empty data.
      await consumer.doThing(ethers.constants.AddressZero, 1)
      expect(await consumer.lastResult()).to.equal(false)
    })

    it("clears a whenCalledWith entry", async () => {
      await target.readValue.whenCalledWith(7).returns(700)
      await target.readValue.reset()

      expect(await consumer.readValueThroughStaticCall(7)).to.equal(0)
    })

    it("clears a view function's configuration, which is never recorded", async () => {
      // `reset` cannot infer entries from recorded calls, because STATICCALL
      // calls are not recorded. It has to track what was configured.
      await target.readValue.returns(5)
      await consumer.readValueThroughStaticCall(7)

      await target.readValue.reset()

      expect(await consumer.readValueThroughStaticCall(7)).to.equal(0)
    })

    it("leaves other functions alone", async () => {
      await target.readValue.returns(5)
      await target.doThing.returns(true)

      await target.doThing.reset()

      expect(await consumer.readValueThroughStaticCall(1)).to.equal(5)
    })

    it("clears everything at the mock level", async () => {
      await target.readValue.returns(5)
      await consumer.noReturn(1)

      await target.reset()

      expect(await consumer.readValueThroughStaticCall(1)).to.equal(0)
      expect(await target.noReturn.callCount()).to.equal(0)
    })
  })

  describe("wallet", () => {
    it("sends transactions from the mock's own address", async () => {
      // smock's `fake.wallet`, used where production code checks
      // `msg.sender == someContract`.
      const factory = await ethers.getContractFactory("MockTargetConsumer")
      const other = await factory.deploy(target.address)
      await other.deployed()

      const tx = await other.connect(target.wallet).noReturn(1)
      const receipt = await tx.wait()

      expect(receipt.from).to.equal(target.address)
    })
  })

  describe("address option", () => {
    it("deploys at a requested address", async () => {
      const address = ethers.utils.getAddress(
        `0x${"ab".repeat(20)}`.toLowerCase()
      )

      const pinned = await createMock<IMockTarget>("IMockTarget", { address })

      expect(pinned.address).to.equal(address)
      await pinned.readValue.returns(7)

      const factory = await ethers.getContractFactory("MockTargetConsumer")
      const pinnedConsumer = await factory.deploy(address)
      await pinnedConsumer.deployed()

      expect(await pinnedConsumer.readValueThroughStaticCall(1)).to.equal(7)
    })
  })

  describe("storage left behind at a pinned address", () => {
    it("is not read back as configuration", async () => {
      // Mocks are routinely installed over an address that already holds a
      // deployed contract — `test/fixtures/bridge.ts` pins them at the Bridge's
      // real ecdsaWalletRegistry and relay. `hardhat_setCode` replaces the code
      // and leaves the storage, so a mock keeping its state at slots 0, 1, 2...
      // would read that leftover as its own.
      const address = ethers.utils.getAddress(`0x${"cd".repeat(20)}`)
      const garbage =
        "0xdeadbeef00000000000000000000000000000000000000000000000000000001"

      await Promise.all(
        Array.from({ length: 8 }, (_, slot) =>
          ethers.provider.send("hardhat_setStorageAt", [
            address,
            ethers.utils.hexValue(slot),
            garbage,
          ])
        )
      )

      const pinned = await createMock<IMockTarget>("IMockTarget", { address })

      expect(await pinned.doThing.callCount()).to.equal(0)

      await pinned.readValue.returns(7)

      const factory = await ethers.getContractFactory("MockTargetConsumer")
      const pinnedConsumer = await factory.deploy(address)
      await pinnedConsumer.deployed()

      expect(await pinnedConsumer.readValueThroughStaticCall(1)).to.equal(7)
    })
  })

  describe("call assertions on read-only functions", () => {
    it("are refused rather than silently answered zero", async () => {
      // A view function is reached by STATICCALL and cannot be recorded.
      // Answering "0 calls" would read as a passing assertion.
      let message = ""
      try {
        await target.readValue.callCount()
      } catch (error) {
        message = (error as Error).message
      }

      expect(message).to.contain("STATICCALL")
    })

    it("still allow the function to be stubbed", async () => {
      await target.readValue.returns(3)

      expect(await consumer.readValueThroughStaticCall(1)).to.equal(3)
    })
  })
})

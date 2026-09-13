import { ethers } from "hardhat"
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers"
import { expect } from "chai"

import type { TestAltBn128 } from "../typechain"

describe("AltBn128", () => {
  const g1 =
    "0x15c30f4b6cf6dbbcbdcc10fe22f54c8170aea44e198139b776d512d8f027319a1b9e8bfaf1383978231ce98e42bafc8129f473fc993cf60ce327f7d223460663"
  const g2 =
    "0x1f1954b33144db2b5c90da089e8bde287ec7089d5d6433f3b6becaefdb678b1b2a9de38d14bef2cf9afc3c698a4211fa7ada7b4f036a2dfef0dc122b423259d01659dc18b57722ecf6a4beb4d04dfe780a660c4c3bb2b165ab8486114c464c621bf37ecdba226629c20908c7f475c5b3a7628ce26d696436eab0b0148034dfcd"
  const g2Compressed =
    "0x1f1954b33144db2b5c90da089e8bde287ec7089d5d6433f3b6becaefdb678b1b2a9de38d14bef2cf9afc3c698a4211fa7ada7b4f036a2dfef0dc122b423259d0"

  let testAltBn128: TestAltBn128

  const fixture = async () => {
    const TestAltBn128 = await ethers.getContractFactory("TestAltBn128")
    testAltBn128 = await TestAltBn128.deploy()
    await testAltBn128.deployed()

    return testAltBn128
  }

  beforeEach("load test fixture", async () => {
    testAltBn128 = await loadFixture(fixture)
  })

  describe("g1Unmarshal", async () => {
    it("should not accept less than 64 bytes", async () => {
      await expect(
        testAltBn128.publicG1Unmarshal(g1.slice(0, -2)),
      ).to.be.revertedWith("Invalid G1 bytes length")
    })

    it("should accept 64 bytes", async () => {
      await testAltBn128.publicG1Unmarshal(g1)
      // ok, no revert
    })

    it("should not accept more than 64 bytes", async () => {
      await expect(
        testAltBn128.publicG1Unmarshal(`${g1}ff`),
      ).to.be.revertedWith("Invalid G1 bytes length")
    })
  })

  describe("g2Unmarshal", async () => {
    it("should not accept less than 128 bytes", async () => {
      await expect(
        testAltBn128.publicG2Unmarshal(g2.slice(0, -2)),
      ).to.be.revertedWith("Invalid G2 bytes length")
    })

    it("should accept 128 bytes", async () => {
      await testAltBn128.publicG2Unmarshal(g2)
      // ok, no revert
    })

    it("should not accept more than 128 bytes", async () => {
      await expect(
        testAltBn128.publicG2Unmarshal(`${g2}ff`),
      ).to.be.revertedWith("Invalid G2 bytes length")
    })
  })

  describe("g2Decompress", async () => {
    it("should not accept less than 64 bytes", async () => {
      await expect(
        testAltBn128.publicG2Decompress(g2Compressed.slice(0, -2)),
      ).to.be.revertedWith("Invalid G2 compressed bytes length")
    })

    it("should accept 64 bytes", async () => {
      await testAltBn128.publicG2Decompress(g2Compressed)
      // ok, no revert
    })

    it("should not accept more than 64 bytes", async () => {
      await expect(
        testAltBn128.publicG2Decompress(`${g2Compressed}ff`),
      ).to.be.revertedWith("Invalid G2 compressed bytes length")
    })
  })

  it("runHashingTest()", async () => {
    await testAltBn128.runHashingTest()
    // ok, no revert
  })

  it("runHashAndAddTest()", async () => {
    await testAltBn128.runHashAndAddTest()
    // ok, no revert
  })

  it("runHashAndScalarMultiplyTest()", async () => {
    await testAltBn128.runHashAndScalarMultiplyTest()
    // ok, no revert
  })

  it("runGfP2AddTest()", async () => {
    await testAltBn128.runGfP2AddTest()
    // ok, no revert
  })

  it("runAddTest()", async () => {
    await testAltBn128.runAddTest()
    // ok, no revert
  })

  it("runScalarMultiplyTest()", async () => {
    await testAltBn128.runScalarMultiplyTest()
    // ok, no revert
  })

  it("runBasicPairingTest()", async () => {
    await testAltBn128.runBasicPairingTest()
    // ok, no revert
  })

  it("runG1PointMarshalingTest()", async () => {
    await testAltBn128.runG1PointMarshalingTest()
    // ok, no revert
  })

  it("runVerifySignatureTest()", async () => {
    await testAltBn128.runVerifySignatureTest()
    // ok, no revert
  })

  it("runCompressG1InvertibilityTest()", async () => {
    await testAltBn128.runCompressG1InvertibilityTest()
    // ok, no revert
  })

  it("runCompressG2InvertibilityTest()", async () => {
    await testAltBn128.runCompressG2InvertibilityTest()
    // ok, no revert
  })

  it("runG2PointOnCurveTest()", async () => {
    await testAltBn128.runG2PointOnCurveTest()
    // ok, no revert
  })
})

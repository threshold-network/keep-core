import { waffle } from "hardhat"
import { expect } from "chai"

import blsData from "./data/bls"
import { blsDeployment } from "./fixtures"

import type { BLS } from "../typechain"

describe("BLS", () => {
  let bls: BLS

  beforeEach("load test fixture", async () => {
    const contracts = await waffle.loadFixture(blsDeployment)

    bls = contracts.bls as BLS
  })

  it("should be able to verify BLS signature", async () => {
    // Corresponding test in Go library: bls_test.go TestSignAndVerifyG1
    const result = await bls.verify(
      "0x1f1954b33144db2b5c90da089e8bde287ec7089d5d6433f3b6becaefdb678b1b2a9de38d14bef2cf9afc3c698a4211fa7ada7b4f036a2dfef0dc122b423259d01659dc18b57722ecf6a4beb4d04dfe780a660c4c3bb2b165ab8486114c464c621bf37ecdba226629c20908c7f475c5b3a7628ce26d696436eab0b0148034dfcd",
      "0x15c30f4b6cf6dbbcbdcc10fe22f54c8170aea44e198139b776d512d8f027319a1b9e8bfaf1383978231ce98e42bafc8129f473fc993cf60ce327f7d223460663",
      "0x112d462728e89432b0fe40251eeb6608aed4560f3dc833a9877f5010ace9b1312006dbbe2f30c6e0e3e7ec47dc078b7b6b773379d44d64e44ec4e017bfa7375c",
    )
    await expect(result).to.be.true
  })

  it("should be able to verify threshold BLS recovered/reconstructed signature", async () => {
    // Corresponding test in Go library: bls_test.go TestThresholdBLS
    const result = await bls.verify(
      "0x1644bcbb604e3608225d1826bab0b926f2df4fb506e1aa3641d5ab350ebceb5825c7df94f3a87e9dd6e11865dfdbdd3db69eab4951c8bc2250fb51da5f813009131e0c9e6d90d91741458b522b57ca99b597dd922dd31f61a2f69412ce3220d31a1ec4b09ef2ea1d6ba7cad98386f6049b5eec5fb3a40408229dc75c5759f184",
      "0x15c30f4b6cf6dbbcbdcc10fe22f54c8170aea44e198139b776d512d8f027319a1b9e8bfaf1383978231ce98e42bafc8129f473fc993cf60ce327f7d223460663",
      "0x23cbfa4b2fcbf43a44d8a4b2a9aa1a9123f183794fa7b53c633c2de7ada5b5ca174f81900dc4ca5672768d51c12dfcb0eac2aafba0a66ac54b76f689dc1fe321",
    )
    await expect(result).to.be.true
  })

  it("should use reasonable amount of gas", async () => {
    // Corresponding test in Go library: bls_test.go TestThresholdBLS
    const gasEstimate = await bls.estimateGas.verify(
      "0x1644bcbb604e3608225d1826bab0b926f2df4fb506e1aa3641d5ab350ebceb5825c7df94f3a87e9dd6e11865dfdbdd3db69eab4951c8bc2250fb51da5f813009131e0c9e6d90d91741458b522b57ca99b597dd922dd31f61a2f69412ce3220d31a1ec4b09ef2ea1d6ba7cad98386f6049b5eec5fb3a40408229dc75c5759f184",
      "0x15c30f4b6cf6dbbcbdcc10fe22f54c8170aea44e198139b776d512d8f027319a1b9e8bfaf1383978231ce98e42bafc8129f473fc993cf60ce327f7d223460663",
      "0x23cbfa4b2fcbf43a44d8a4b2a9aa1a9123f183794fa7b53c633c2de7ada5b5ca174f81900dc4ca5672768d51c12dfcb0eac2aafba0a66ac54b76f689dc1fe321",
    )
    // make sure no change will make the verification more expensive than it is now
    await expect(gasEstimate.toNumber()).to.be.lessThan(
      306682,
      "BLS verification is too expensive",
    )
  })

  it("should be able to verify BLS aggregated signature", async () => {
    // Corresponding test in Go library: bls_test.go TestAggregateBLS
    const result = await bls.verify(
      "0x04ab0e5862ecdffda6bab4465c4ee88a3b71a86f178c1ac6e89a4827464c618215f83a353b5ba5126f7fdfb21998fb36d1169db87ea4042ac0d60106c98c9b8122c158a3411a0ea19841c60bcc1da84cf94f5959f1783d7ee751a48d909f58f10bbcfc4acb66b369e61c91b3a5620167ab861a80c639d1fd14b2414cd386853b",
      "0x15c30f4b6cf6dbbcbdcc10fe22f54c8170aea44e198139b776d512d8f027319a1b9e8bfaf1383978231ce98e42bafc8129f473fc993cf60ce327f7d223460663",
      "0x0855a2afab929270bd423e0d4069250519a45e4c3bcb33f53531f5b6988bb87b14301047405783a8d52311f4dfebe6a8f5acc7f299cf576c38cf726bc9fc0a1a",
    )
    await expect(result).to.be.true
  })

  it("should fail to verify invalid BLS signature", async () => {
    const result = await bls.verify(
      "0x1f1954b33144db2b5c90da089e8bde287ec7089d5d6433f3b6becaefdb678b1b2a9de38d14bef2cf9afc3c698a4211fa7ada7b4f036a2dfef0dc122b423259d01659dc18b57722ecf6a4beb4d04dfe780a660c4c3bb2b165ab8486114c464c621bf37ecdba226629c20908c7f475c5b3a7628ce26d696436eab0b0148034dfcd",
      "0x15c30f4b6cf6dbbcbdcc10fe22f54c8170aea44e198139b776d512d8f027319a1b9e8bfaf1383978231ce98e42bafc8129f473fc993cf60ce327f7d223460663",
      "0x0855a2afab929270bd423e0d4069250519a45e4c3bcb33f53531f5b6988bb87b14301047405783a8d52311f4dfebe6a8f5acc7f299cf576c38cf726bc9fc0a1a",
    )
    await expect(result).to.be.false
  })

  it("should fail to verify BLS signature without valid message", async () => {
    const result = await bls.verify(
      "0x1f1954b33144db2b5c90da089e8bde287ec7089d5d6433f3b6becaefdb678b1b2a9de38d14bef2cf9afc3c698a4211fa7ada7b4f036a2dfef0dc122b423259d01659dc18b57722ecf6a4beb4d04dfe780a660c4c3bb2b165ab8486114c464c621bf37ecdba226629c20908c7f475c5b3a7628ce26d696436eab0b0148034dfcd",
      "0x1a01114fce4c287d8beb49616ca8f2c2be211820b73340c79bd4aada0c4f66af1bcbbb9c398c87dc504e9d275b6f5f97215a081a85d3161910158b4ab331f7bc",
      "0x112d462728e89432b0fe40251eeb6608aed4560f3dc833a9877f5010ace9b1312006dbbe2f30c6e0e3e7ec47dc078b7b6b773379d44d64e44ec4e017bfa7375c",
    )
    await expect(result).to.be.false
  })

  it("should fail to verify BLS signature without valid public key", async () => {
    const result = await bls.verify(
      "0x1644bcbb604e3608225d1826bab0b926f2df4fb506e1aa3641d5ab350ebceb5825c7df94f3a87e9dd6e11865dfdbdd3db69eab4951c8bc2250fb51da5f813009131e0c9e6d90d91741458b522b57ca99b597dd922dd31f61a2f69412ce3220d31a1ec4b09ef2ea1d6ba7cad98386f6049b5eec5fb3a40408229dc75c5759f184",
      "0x15c30f4b6cf6dbbcbdcc10fe22f54c8170aea44e198139b776d512d8f027319a1b9e8bfaf1383978231ce98e42bafc8129f473fc993cf60ce327f7d223460663",
      "0x112d462728e89432b0fe40251eeb6608aed4560f3dc833a9877f5010ace9b1312006dbbe2f30c6e0e3e7ec47dc078b7b6b773379d44d64e44ec4e017bfa7375c",
    )
    await expect(result).to.be.false
  })

  it("should be able to sign a message and verify it", async () => {
    const message =
      "0x4120626561722077616c6b7320696e746f206120626172203132332e2e2e"

    const signature = await bls.sign(message, blsData.secretKey)

    const actual = await bls.verifyBytes(
      blsData.groupPubKey,
      message,
      signature,
    )
    await expect(actual).to.be.true
  })

  describe("verify", async () => {
    it("should fail for public key having less than 128 bytes", async () => {
      await await expect(
        bls.verify(
          "0x1f1954b33144db2b5c90da089e8bde287ec7089d5d6433f3b6becaefdb678b1b2a9de38d14bef2cf9afc3c698a4211fa7ada7b4f036a2dfef0dc122b423259d01659dc18b57722ecf6a4beb4d04dfe780a660c4c3bb2b165ab8486114c464c621bf37ecdba226629c20908c7f475c5b3a7628ce26d696436eab0b0148034df",
          "0x15c30f4b6cf6dbbcbdcc10fe22f54c8170aea44e198139b776d512d8f027319a1b9e8bfaf1383978231ce98e42bafc8129f473fc993cf60ce327f7d223460663",
          "0x112d462728e89432b0fe40251eeb6608aed4560f3dc833a9877f5010ace9b1312006dbbe2f30c6e0e3e7ec47dc078b7b6b773379d44d64e44ec4e017bfa7375c",
        ),
        // TODO: The exact message should be "Invalid G2 bytes length" but due to
        //       https://github.com/nomiclabs/hardhat/issues/1873 the message
        //       is "library was called directly". Until it's resolved, we don't
        //       assert about the message.
      ).to.be.reverted
    })

    it("should fail for public key having more than 128 bytes", async () => {
      await await expect(
        bls.verify(
          "0x1f1954b33144db2b5c90da089e8bde287ec7089d5d6433f3b6becaefdb678b1b2a9de38d14bef2cf9afc3c698a4211fa7ada7b4f036a2dfef0dc122b423259d01659dc18b57722ecf6a4beb4d04dfe780a660c4c3bb2b165ab8486114c464c621bf37ecdba226629c20908c7f475c5b3a7628ce26d696436eab0b0148034dfcdcd",
          "0x15c30f4b6cf6dbbcbdcc10fe22f54c8170aea44e198139b776d512d8f027319a1b9e8bfaf1383978231ce98e42bafc8129f473fc993cf60ce327f7d223460663",
          "0x112d462728e89432b0fe40251eeb6608aed4560f3dc833a9877f5010ace9b1312006dbbe2f30c6e0e3e7ec47dc078b7b6b773379d44d64e44ec4e017bfa7375c",
        ),
        // TODO: The exact message should be "Invalid G2 bytes length" but due to
        //       https://github.com/nomiclabs/hardhat/issues/1873 the message
        //       is "library was called directly". Until it's resolved, we don't
        //       assert about the message.
      ).to.be.reverted
    })

    it("should fail for message having less than 64 bytes", async () => {
      await await expect(
        bls.verify(
          "0x1f1954b33144db2b5c90da089e8bde287ec7089d5d6433f3b6becaefdb678b1b2a9de38d14bef2cf9afc3c698a4211fa7ada7b4f036a2dfef0dc122b423259d01659dc18b57722ecf6a4beb4d04dfe780a660c4c3bb2b165ab8486114c464c621bf37ecdba226629c20908c7f475c5b3a7628ce26d696436eab0b0148034dfcd",
          "0x15c30f4b6cf6dbbcbdcc10fe22f54c8170aea44e198139b776d512d8f027319a1b9e8bfaf1383978231ce98e42bafc8129f473fc993cf60ce327f7d2234606",
          "0x112d462728e89432b0fe40251eeb6608aed4560f3dc833a9877f5010ace9b1312006dbbe2f30c6e0e3e7ec47dc078b7b6b773379d44d64e44ec4e017bfa7375c",
        ),
        // TODO: The exact message should be "Invalid G1 bytes length" but due to
        //       https://github.com/nomiclabs/hardhat/issues/1873 the message
        //       is "library was called directly". Until it's resolved, we don't
        //       assert about the message.
      ).to.be.reverted
    })

    it("should fail for message having more than 64 bytes", async () => {
      await await expect(
        bls.verify(
          "0x1f1954b33144db2b5c90da089e8bde287ec7089d5d6433f3b6becaefdb678b1b2a9de38d14bef2cf9afc3c698a4211fa7ada7b4f036a2dfef0dc122b423259d01659dc18b57722ecf6a4beb4d04dfe780a660c4c3bb2b165ab8486114c464c621bf37ecdba226629c20908c7f475c5b3a7628ce26d696436eab0b0148034dfcd",
          "0x15c30f4b6cf6dbbcbdcc10fe22f54c8170aea44e198139b776d512d8f027319a1b9e8bfaf1383978231ce98e42bafc8129f473fc993cf60ce327f7d22346066363",
          "0x112d462728e89432b0fe40251eeb6608aed4560f3dc833a9877f5010ace9b1312006dbbe2f30c6e0e3e7ec47dc078b7b6b773379d44d64e44ec4e017bfa7375c",
        ),
        // TODO: The exact message should be "Invalid G1 bytes length" but due to
        //       https://github.com/nomiclabs/hardhat/issues/1873 the message
        //       is "library was called directly". Until it's resolved, we don't
        //       assert about the message.
      ).to.be.reverted
    })

    it("should fail for signature having less than 64 bytes", async () => {
      await await expect(
        bls.verify(
          "0x1f1954b33144db2b5c90da089e8bde287ec7089d5d6433f3b6becaefdb678b1b2a9de38d14bef2cf9afc3c698a4211fa7ada7b4f036a2dfef0dc122b423259d01659dc18b57722ecf6a4beb4d04dfe780a660c4c3bb2b165ab8486114c464c621bf37ecdba226629c20908c7f475c5b3a7628ce26d696436eab0b0148034dfcd",
          "0x15c30f4b6cf6dbbcbdcc10fe22f54c8170aea44e198139b776d512d8f027319a1b9e8bfaf1383978231ce98e42bafc8129f473fc993cf60ce327f7d223460663",
          "0x112d462728e89432b0fe40251eeb6608aed4560f3dc833a9877f5010ace9b1312006dbbe2f30c6e0e3e7ec47dc078b7b6b773379d44d64e44ec4e017bfa737",
        ),
        // TODO: The exact message should be "Invalid G1 bytes length" but due to
        //       https://github.com/nomiclabs/hardhat/issues/1873 the message
        //       is "library was called directly". Until it's resolved, we don't
        //       assert about the message.
      ).to.be.reverted
    })

    it("should fail for signature having more than 64 bytes", async () => {
      await await expect(
        bls.verify(
          "0x1f1954b33144db2b5c90da089e8bde287ec7089d5d6433f3b6becaefdb678b1b2a9de38d14bef2cf9afc3c698a4211fa7ada7b4f036a2dfef0dc122b423259d01659dc18b57722ecf6a4beb4d04dfe780a660c4c3bb2b165ab8486114c464c621bf37ecdba226629c20908c7f475c5b3a7628ce26d696436eab0b0148034dfcd",
          "0x15c30f4b6cf6dbbcbdcc10fe22f54c8170aea44e198139b776d512d8f027319a1b9e8bfaf1383978231ce98e42bafc8129f473fc993cf60ce327f7d223460663",
          "0x112d462728e89432b0fe40251eeb6608aed4560f3dc833a9877f5010ace9b1312006dbbe2f30c6e0e3e7ec47dc078b7b6b773379d44d64e44ec4e017bfa7375c5c",
        ),
        // TODO: The exact message should be "Invalid G1 bytes length" but due to
        //       https://github.com/nomiclabs/hardhat/issues/1873 the message
        //       is "library was called directly". Until it's resolved, we don't
        //       assert about the message.
      ).to.be.reverted
    })
  })
})

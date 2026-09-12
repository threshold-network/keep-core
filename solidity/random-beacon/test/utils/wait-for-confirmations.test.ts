import assert from "assert/strict"
import { ethers } from "hardhat"
import { expect } from "chai"
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers"

import waitForConfirmations from "../../utils/wait-for-confirmations"

async function sentTransaction() {
  const [signer] = await ethers.getSigners()
  return signer.sendTransaction({ to: signer.address, value: 0 })
}

describe("deployment confirmations", () => {
  it("retrieves a confirmed transaction through the Hardhat ethers v6 provider", async () => {
    const transaction = await loadFixture(sentTransaction)
    const receipt = await waitForConfirmations(
      ethers.provider,
      transaction.hash,
      1,
      5_000,
    )
    expect(receipt.hash).to.equal(transaction.hash)
    expect(receipt.status).to.equal(1)
  })

  it("waits for the requested number of confirmations", async () => {
    const transaction = await loadFixture(sentTransaction)
    const pending = waitForConfirmations(
      ethers.provider,
      transaction.hash,
      2,
      5_000,
    )
    await ethers.provider.send("evm_mine", [])
    const receipt = await pending
    expect(await receipt.confirmations()).to.be.at.least(2)
  })

  it("fails if the additional confirmation does not arrive before the timeout", async () => {
    const transaction = await loadFixture(sentTransaction)
    await assert.rejects(
      waitForConfirmations(ethers.provider, transaction.hash, 2, 50),
      (error: unknown) => ethers.isError(error, "TIMEOUT"),
    )
  })

  it("fails if the saved deployment transaction cannot be found", async () => {
    await assert.rejects(
      waitForConfirmations(ethers.provider, ethers.ZeroHash, 1, 5_000),
      /Deployment transaction .* was not found/,
    )
  })
})

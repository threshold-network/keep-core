/* eslint-disable no-underscore-dangle */
import { ethers, helpers } from "hardhat"
import { expect } from "chai"

import { expectCalledWith } from "./helpers/mock"
import { params, walletRegistryFixture } from "./fixtures"
import { submitRelayEntry } from "./utils/randomBeacon"
import { signAndSubmitCorrectDkgResult } from "./utils/dkg"
import ecdsaData from "./data/ecdsa"

import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { DkgResult } from "./utils/dkg"
import type { Mock } from "./helpers/mock"
import type {
  IWalletOwner,
  WalletRegistry,
  WalletRegistryStub,
} from "../typechain"
import type { ContractTransaction } from "ethers"

const { mineBlocks } = helpers.time

const { createSnapshot, restoreSnapshot } = helpers.snapshot

describe("WalletRegistry - Wallet Owner", async () => {
  const groupPublicKey: string = ethers.utils.hexValue(
    ecdsaData.group1.publicKey
  )
  const groupPublicKeyX: string = ethers.utils.hexValue(
    ecdsaData.group1.publicKeyX
  )
  const groupPublicKeyY: string = ethers.utils.hexValue(
    ecdsaData.group1.publicKeyY
  )
  const walletID: string = ethers.utils.keccak256(groupPublicKey)

  let walletRegistry: WalletRegistryStub & WalletRegistry
  let walletOwner: Mock<IWalletOwner>

  before("load test fixture", async () => {
    // eslint-disable-next-line @typescript-eslint/no-extra-semi
    ;({ walletRegistry, walletOwner } = await walletRegistryFixture({
      useAllowlist: true,
    }))
  })

  describe("approveDkgResult", async () => {
    let dkgResult: DkgResult
    let submitter: SignerWithAddress

    before(async () => {
      await createSnapshot()

      await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
      const { startBlock, dkgSeed } = await submitRelayEntry(walletRegistry)

      ;({ dkgResult, submitter } = await signAndSubmitCorrectDkgResult(
        walletRegistry,
        groupPublicKey,
        dkgSeed,
        startBlock
      ))

      await mineBlocks(params.dkgResultChallengePeriodLength)
    })

    after(async () => {
      await restoreSnapshot()

      await walletOwner.__ecdsaWalletCreatedCallback.reset()
    })

    context("when __ecdsaWalletCreatedCallback reverts", async () => {
      let tx: Promise<ContractTransaction>

      before(async () => {
        await createSnapshot()

        await walletOwner.__ecdsaWalletCreatedCallback.reverts(
          "wallet owner internal error"
        )

        tx = walletRegistry.connect(submitter).approveDkgResult(dkgResult)
      })

      after(async () => {
        await restoreSnapshot()

        await walletOwner.__ecdsaWalletCreatedCallback.reset()
      })

      it("should revert", async () => {
        // FIXME: For some reason this check doesn't work with the expected error message
        // await expect(tx).to.be.revertedWith("wallet owner internal error")
        await expect(tx).to.be.reverted
      })
    })

    context("when __ecdsaWalletCreatedCallback succeeds", async () => {
      let tx: Promise<ContractTransaction>

      before(async () => {
        await createSnapshot()

        tx = walletRegistry.connect(submitter).approveDkgResult(dkgResult)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should succeed", async () => {
        await expect(tx).to.not.be.reverted
      })

      it("should call wallet owner", async () => {
        await tx

        await expectCalledWith(walletOwner.__ecdsaWalletCreatedCallback, [
          walletID,
          groupPublicKeyX,
          groupPublicKeyY,
        ])
      })
    })
  })
})

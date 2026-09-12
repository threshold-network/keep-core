/* eslint-disable @typescript-eslint/no-extra-semi */

import { ethers, waffle, helpers } from "hardhat"
import { expect } from "chai"

import blsData from "./data/bls"
import { constants, params, randomBeaconDeployment } from "./fixtures"
import { createGroup } from "./utils/groups"
import { registerOperators } from "./utils/operators"

import type { DeployedContracts } from "./fixtures"
import type {
  RandomBeaconStub,
  T,
  CallbackContractStub,
  RandomBeacon,
  RandomBeaconGovernance,
} from "../typechain"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"

const ZERO_ADDRESS = ethers.constants.AddressZero
const { createSnapshot, restoreSnapshot } = helpers.snapshot

const fixture = async () => {
  const deployment = await randomBeaconDeployment()

  const contracts: DeployedContracts = {
    randomBeacon: deployment.randomBeacon,
    randomBeaconGovernance: deployment.randomBeaconGovernance,
    t: deployment.t,
    callbackContractStub: await (
      await ethers.getContractFactory("CallbackContractStub")
    ).deploy(),
    callbackContractStub1: await (
      await ethers.getContractFactory("CallbackContractStub")
    ).deploy(),
  }

  // Accounts offset provided to slice getUnnamedSigners have to include number
  // of unnamed accounts that were already used.
  const signers = await registerOperators(
    contracts.randomBeacon as RandomBeacon,
    contracts.t as T,
    constants.groupSize,
    2
  )

  await createGroup(contracts.randomBeacon as RandomBeacon, signers)

  return { contracts, signers }
}

describe("RandomBeacon - Callback", () => {
  let requester: SignerWithAddress
  let submitter: SignerWithAddress
  let governance: SignerWithAddress

  let randomBeacon: RandomBeaconStub
  let randomBeaconGovernance: RandomBeaconGovernance
  let t: T
  let callbackContract: CallbackContractStub
  let callbackContract1: CallbackContractStub

  before(async () => {
    ;[requester, submitter] = await helpers.signers.getUnnamedSigners()
    ;({ governance } = await helpers.signers.getNamedSigners())

    const { contracts } = await waffle.loadFixture(fixture)

    randomBeacon = contracts.randomBeacon as RandomBeaconStub
    randomBeaconGovernance =
      contracts.randomBeaconGovernance as RandomBeaconGovernance
    t = contracts.t as T
    callbackContract = contracts.callbackContractStub as CallbackContractStub
    callbackContract1 = contracts.callbackContractStub1 as CallbackContractStub

    await randomBeaconGovernance
      .connect(governance)
      .setRequesterAuthorization(requester.address, true)
  })

  describe("requestRelayEntry", () => {
    before(async () => {
      await createSnapshot()
    })

    after(async () => {
      await restoreSnapshot()
    })

    context("when passed non-zero and zero callback addresses", () => {
      it("should be set to a non-zero callback contract address", async () => {
        await createSnapshot()

        await randomBeacon
          .connect(requester)
          .requestRelayEntry(callbackContract.address)

        await expect(await randomBeacon.getCallbackContract()).to.equal(
          callbackContract.address
        )

        await restoreSnapshot()
      })

      it("should reset to zero callback address", async () => {
        await createSnapshot()

        await randomBeacon
          .connect(requester)
          .requestRelayEntry(callbackContract.address)

        await randomBeacon
          .connect(submitter)
          ["submitRelayEntry(bytes)"](blsData.groupSignature)

        await randomBeacon.connect(requester).requestRelayEntry(ZERO_ADDRESS)

        await expect(await randomBeacon.getCallbackContract()).to.equal(
          ZERO_ADDRESS
        )

        await restoreSnapshot()
      })

      it("should be set to the latest non-zero callback address", async () => {
        await createSnapshot()

        await randomBeacon
          .connect(requester)
          .requestRelayEntry(callbackContract.address)

        await randomBeacon
          .connect(submitter)
          ["submitRelayEntry(bytes)"](blsData.groupSignature)

        await randomBeacon
          .connect(requester)
          .requestRelayEntry(callbackContract1.address)

        await expect(await randomBeacon.getCallbackContract()).to.equal(
          callbackContract1.address
        )

        await restoreSnapshot()
      })
    })
  })

  describe("submitRelayEntry", () => {
    before(async () => {
      await createSnapshot()
    })

    after(async () => {
      await restoreSnapshot()
    })

    context("when the callback is set", () => {
      context("when the callback was executed", () => {
        it("should set callback contract params", async () => {
          await createSnapshot()

          await randomBeacon
            .connect(requester)
            .requestRelayEntry(callbackContract.address)

          await randomBeacon
            .connect(submitter)
            ["submitRelayEntry(bytes)"](blsData.groupSignature)

          const lastEntry = await callbackContract.lastEntry()
          await expect(lastEntry).to.equal(blsData.groupSignatureUint256)

          const blockNumber = await callbackContract.blockNumber()
          const latestBlock = await ethers.provider.getBlock("latest")

          await expect(blockNumber).to.equal(latestBlock.number)

          await restoreSnapshot()
        })
      })

      context("when the callback failed", () => {
        it("should emit a callback failed event because of the gas limit", async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginCallbackGasLimitUpdate(40000)
          await helpers.time.increaseTime(params.governanceDelay)
          await randomBeaconGovernance
            .connect(governance)
            .finalizeCallbackGasLimitUpdate()

          await randomBeacon
            .connect(requester)
            .requestRelayEntry(callbackContract.address)

          const tx = await randomBeacon
            .connect(submitter)
            ["submitRelayEntry(bytes)"](blsData.groupSignature)

          await expect(tx)
            .to.emit(randomBeacon, "CallbackFailed")
            .withArgs(blsData.groupSignatureUint256, tx.blockNumber)

          await restoreSnapshot()
        })

        it("should emit a callback failed event because of the internal error", async () => {
          await createSnapshot()

          await randomBeacon
            .connect(requester)
            .requestRelayEntry(callbackContract.address)

          await callbackContract.setFailureFlag(true)

          const tx = await randomBeacon
            .connect(submitter)
            ["submitRelayEntry(bytes)"](blsData.groupSignature)

          await expect(tx)
            .to.emit(randomBeacon, "CallbackFailed")
            .withArgs(blsData.groupSignatureUint256, tx.blockNumber)

          await restoreSnapshot()
        })
      })
    })
  })
})

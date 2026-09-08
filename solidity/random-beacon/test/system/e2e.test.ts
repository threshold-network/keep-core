import { ethers, waffle, helpers } from "hardhat"
import { expect } from "chai"

import {
  constants,
  dkgState,
  params,
  randomBeaconDeployment,
} from "../fixtures"
import {
  genesis,
  signAndSubmitCorrectDkgResult,
  noMisbehaved,
} from "../utils/dkg"
import blsData from "../data/bls"
import { registerOperators } from "../utils/operators"

import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type {
  RandomBeacon,
  RandomBeaconStub,
  T,
  RandomBeaconGovernance,
} from "../../typechain"

const ZERO_ADDRESS = ethers.constants.AddressZero

const { mineBlocks, mineBlocksTo } = helpers.time
const { keccak256 } = ethers.utils

const fixture = async () => {
  const contracts = await randomBeaconDeployment()

  // Accounts offset provided to slice getUnnamedSigners have to include number
  // of unnamed accounts that were already used.
  await registerOperators(
    contracts.randomBeacon as RandomBeacon,
    contracts.t as T,
    constants.groupSize,
    1
  )

  const randomBeacon = contracts.randomBeacon as RandomBeaconStub & RandomBeacon
  const randomBeaconGovernance =
    contracts.randomBeaconGovernance as RandomBeaconGovernance
  const t = contracts.t as T

  return {
    randomBeacon,
    randomBeaconGovernance,
    t,
  }
}

// End to end test case validating the random beacon generation. This test case
// starts from the genesis call which seeds the initial value (pi) and creates a
// new signing group. The next steps call the random beacon relay requests and
// validate the results of the submitted signatures. At the end of this scenario
// 3 active groups should be added to the chain as a result of signatures submission
// and dkg under the hood.
// Group creation frequency is adjusted for the purpose of the e2e testing data.
// All the other init params should be intact and use the existing params set in
// in tests fixture.
// Signatures in bls.ts were generated outside of this test based on bls_test.go
describe("System -- e2e", () => {
  const groupCreationFrequency = 5
  const groupPubKeys = [
    blsData.groupPubKey,
    blsData.groupPubKey2,
    blsData.groupPubKey3,
  ]

  let randomBeacon: RandomBeacon
  let randomBeaconGovernance: RandomBeaconGovernance
  let t: T
  let requester: SignerWithAddress
  let governance: SignerWithAddress

  before(async () => {
    const contracts = await waffle.loadFixture(fixture)

    ;({ governance } = await helpers.signers.getNamedSigners())
    ;[requester] = await helpers.signers.getUnnamedSigners()
    randomBeacon = contracts.randomBeacon
    randomBeaconGovernance = contracts.randomBeaconGovernance
    t = contracts.t

    await randomBeaconGovernance
      .connect(governance)
      .beginGroupCreationFrequencyUpdate(groupCreationFrequency)

    await helpers.time.increaseTime(params.governanceDelay)

    await randomBeaconGovernance
      .connect(governance)
      .finalizeGroupCreationFrequencyUpdate()

    await randomBeaconGovernance
      .connect(governance)
      .setRequesterAuthorization(requester.address, true)
  })

  context("when testing a happy path with 15 relay requests", () => {
    let groupPubKeyCounter = 0
    const groupMembers = []

    it("should create 3 new groups", async () => {
      expect(await randomBeacon.getGroupCreationState()).to.be.equal(
        dkgState.IDLE
      )

      const [genesisTx, genesisSeed] = await genesis(randomBeacon)

      expect(await randomBeacon.getGroupCreationState()).to.be.equal(
        dkgState.KEY_GENERATION
      )

      // pass key generation state and transition to awaiting result state
      await mineBlocksTo(
        (await genesisTx.wait()).blockNumber + constants.offchainDkgTime + 1
      )

      expect(await randomBeacon.getGroupCreationState()).to.be.equal(
        dkgState.AWAITING_RESULT
      )

      let dkgResult = await signAndSubmitCorrectDkgResult(
        randomBeacon,
        groupPubKeys[groupPubKeyCounter],
        genesisSeed,
        (
          await genesisTx.wait()
        ).blockNumber,
        noMisbehaved
      )
      groupMembers.push(dkgResult.members)

      await mineBlocks(params.dkgResultChallengePeriodLength)

      expect(await randomBeacon.getGroupCreationState()).to.be.equal(
        dkgState.CHALLENGE
      )

      await randomBeacon
        .connect(dkgResult.submitter)
        .approveDkgResult(dkgResult.dkgResult)

      for (let i = 1; i <= 14; i++) {
        await randomBeacon.connect(requester).requestRelayEntry(ZERO_ADDRESS)

        const txSubmitRelayEntry = await randomBeacon
          .connect(dkgResult.submitter)
          ["submitRelayEntry(bytes)"](blsData.groupSignatures[i - 1])

        // every 5th relay request triggers a new dkg
        if (i % groupCreationFrequency === 0) {
          groupPubKeyCounter += 1
          expect(await randomBeacon.getGroupCreationState()).to.be.equal(
            dkgState.KEY_GENERATION
          )

          await mineBlocksTo(
            (await txSubmitRelayEntry.wait()).blockNumber +
              constants.offchainDkgTime +
              1
          )

          expect(await randomBeacon.getGroupCreationState()).to.be.equal(
            dkgState.AWAITING_RESULT
          )

          dkgResult = await signAndSubmitCorrectDkgResult(
            randomBeacon,
            groupPubKeys[groupPubKeyCounter],
            ethers.BigNumber.from(
              ethers.utils.keccak256(blsData.groupSignatures[i - 1])
            ),
            (
              await txSubmitRelayEntry.wait()
            ).blockNumber,
            noMisbehaved
          )
          groupMembers.push(dkgResult.members)

          await mineBlocks(params.dkgResultChallengePeriodLength)

          expect(await randomBeacon.getGroupCreationState()).to.be.equal(
            dkgState.CHALLENGE
          )

          await randomBeacon
            .connect(dkgResult.submitter)
            .approveDkgResult(dkgResult.dkgResult)

          expect(await randomBeacon.getGroupCreationState()).to.be.equal(
            dkgState.IDLE
          )
        }
      }

      const groupsRegistry = await randomBeacon.getGroupsRegistry()
      expect(groupsRegistry).to.be.lengthOf(3)
      expect(groupsRegistry[0]).to.deep.equal(keccak256(groupPubKeys[0]))
      expect(groupsRegistry[1]).to.deep.equal(keccak256(groupPubKeys[1]))
      expect(groupsRegistry[2]).to.deep.equal(keccak256(groupPubKeys[2]))
    })
  })
})

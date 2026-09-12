import { ethers, helpers } from "hardhat"
import { expect } from "chai"
import { BigNumber } from "ethers"

import type { ContractTransaction } from "ethers"
import type { RandomBeaconChaosnet, CallbackContractStub } from "../typechain"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"

const { createSnapshot, restoreSnapshot } = helpers.snapshot

describe("RandomBeaconChaosnet", () => {
  let requester: SignerWithAddress
  let thirdParty: SignerWithAddress
  let deployer: SignerWithAddress

  let randomBeaconChaosnet: RandomBeaconChaosnet
  let callbackContract: CallbackContractStub

  before(async () => {
    ;[requester, thirdParty] = await helpers.signers.getUnnamedSigners()
    ;({ deployer } = await helpers.signers.getNamedSigners())

    randomBeaconChaosnet = (await (
      await ethers.getContractFactory("RandomBeaconChaosnet")
    )
      .connect(deployer)
      .deploy()) as RandomBeaconChaosnet

    callbackContract = (await (
      await ethers.getContractFactory("CallbackContractStub")
    )
      .connect(deployer)
      .deploy()) as CallbackContractStub
  })

  describe("setRequesterAuthorization", () => {
    context("when called not by the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconChaosnet
            .connect(thirdParty)
            .setRequesterAuthorization(requester.address, true)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when called by the owner", () => {
      context("when requester authorization set to true", () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          tx = await randomBeaconChaosnet
            .connect(deployer)
            .setRequesterAuthorization(requester.address, true)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should mark the address as authorized", async () => {
          expect(
            await randomBeaconChaosnet.authorizedRequesters(requester.address)
          ).to.equal(true)
        })

        it("should emit RequesterAuthorizationUpdated event", async () => {
          await expect(tx)
            .to.emit(randomBeaconChaosnet, "RequesterAuthorizationUpdated")
            .withArgs(requester.address, true)
        })
      })

      context("when requester authorization set to false", () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          // First authorize, then deauthorize the requester
          await randomBeaconChaosnet
            .connect(deployer)
            .setRequesterAuthorization(requester.address, true)

          tx = await randomBeaconChaosnet
            .connect(deployer)
            .setRequesterAuthorization(requester.address, false)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should mark the requester as deauthorized", async () => {
          expect(
            await randomBeaconChaosnet.authorizedRequesters(requester.address)
          ).to.equal(false)
        })

        it("should emit RequesterAuthorizationUpdated event", async () => {
          await expect(tx)
            .to.emit(randomBeaconChaosnet, "RequesterAuthorizationUpdated")
            .withArgs(requester.address, false)
        })
      })
    })
  })

  describe("requestRelayEntry", () => {
    context("when requester is not authorized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconChaosnet
            .connect(thirdParty)
            .requestRelayEntry(callbackContract.address)
        ).to.be.revertedWith("Requester must be authorized")
      })
    })

    context("when requester is authorized", () => {
      context("when called once", () => {
        before(async () => {
          await createSnapshot()

          await randomBeaconChaosnet
            .connect(deployer)
            .setRequesterAuthorization(requester.address, true)

          await randomBeaconChaosnet
            .connect(requester)
            .requestRelayEntry(callbackContract.address)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should execute callback with proper entry", async () => {
          expect(await callbackContract.lastEntry()).to.equal(
            // The entry is keccak-256 of the initial value stored in
            // the RandomBeaconChaosnet contract
            BigNumber.from(
              "86322480231844907215266847458792959757192550318770676212332984" +
                "332154459033029"
            )
          )
        })
      })

      context("when called twice", () => {
        before(async () => {
          await createSnapshot()

          await randomBeaconChaosnet
            .connect(deployer)
            .setRequesterAuthorization(requester.address, true)

          // Ensure the initial value of entry stored in the contract is updated
          // by requesting a relay entry twice.
          await randomBeaconChaosnet
            .connect(requester)
            .requestRelayEntry(callbackContract.address)

          await randomBeaconChaosnet
            .connect(requester)
            .requestRelayEntry(callbackContract.address)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should execute the second callback with proper entry", async () => {
          // The entry is keccak-256 calculated twice on the initial value
          // stored in the RandomBeaconChaosnet contract
          expect(await callbackContract.lastEntry()).to.equal(
            BigNumber.from(
              "45055825411044151981109535788320043556123542984485670123474642" +
                "322436340913380"
            )
          )
        })
      })
    })
  })
})

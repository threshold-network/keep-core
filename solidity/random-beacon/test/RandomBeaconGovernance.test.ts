import { ethers, helpers } from "hardhat"
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers"
import { expect } from "chai"

import requireResult from "./helpers/chain"
import { randomBeaconDeployment, params } from "./fixtures"

import type { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers"
import type { ContractTransactionResponse, Signer } from "ethers"
import type {
  RandomBeacon,
  RandomBeaconGovernance,
  RandomBeaconGovernance__factory,
} from "../typechain"

const { createSnapshot, restoreSnapshot } = helpers.snapshot

const governanceDelay = 604800 // 1 week

const ZERO_ADDRESS = ethers.ZeroAddress

const fixture = async () => {
  const { governance } = await helpers.signers.getNamedSigners()

  const contracts = await randomBeaconDeployment()

  const randomBeacon = contracts.randomBeacon as RandomBeacon
  const randomBeaconGovernance =
    contracts.randomBeaconGovernance as RandomBeaconGovernance

  return { governance, randomBeaconGovernance, randomBeacon }
}

describe("RandomBeaconGovernance", () => {
  let governance: Signer
  let thirdParty: SignerWithAddress
  let thirdPartyContract: SignerWithAddress
  let randomBeacon: RandomBeacon
  let randomBeaconGovernance: RandomBeaconGovernance

  // prettier-ignore
  before(async () => {
    [thirdParty, thirdPartyContract] = await helpers.signers.getUnnamedSigners()
    ;({ governance, randomBeaconGovernance, randomBeacon } =
      await loadFixture(fixture))
  })

  describe("constructor", () => {
    let RandomBeaconGovernance: RandomBeaconGovernance__factory

    before(async () => {
      RandomBeaconGovernance = await ethers.getContractFactory(
        "RandomBeaconGovernance",
      )
    })

    context("when random beacon is 0-address", () => {
      it("should revert", async () => {
        await expect(
          RandomBeaconGovernance.deploy(ZERO_ADDRESS, 1),
        ).to.be.revertedWith("Zero-address reference")
      })
    })

    context("when governance delay is 0", () => {
      it("should revert", async () => {
        await expect(
          RandomBeaconGovernance.deploy(await randomBeacon.getAddress(), 0),
        ).to.be.revertedWith("No governance delay")
      })
    })
  })

  describe("beginGovernanceDelayUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginGovernanceDelayUpdate(1),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginGovernanceDelayUpdate(1337)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the governance delay", async () => {
        expect(await randomBeaconGovernance.governanceDelay()).to.be.equal(
          governanceDelay,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingGovernanceDelayUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit GovernanceDelayUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(randomBeaconGovernance, "GovernanceDelayUpdateStarted")
          .withArgs(1337, blockTimestamp)
      })
    })
  })

  describe("finalizeGovernanceDelayUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeGovernanceDelayUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeGovernanceDelayUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      before(async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginGovernanceDelayUpdate(7331)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeGovernanceDelayUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginGovernanceDelayUpdate(7331)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeGovernanceDelayUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the governance delay", async () => {
          expect(await randomBeaconGovernance.governanceDelay()).to.be.equal(
            7331,
          )
        })

        it("should emit GovernanceDelayUpdated event", async () => {
          await expect(tx)
            .to.emit(randomBeaconGovernance, "GovernanceDelayUpdated")
            .withArgs(7331)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingGovernanceDelayUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginRandomBeaconGovernanceTransfer", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginRandomBeaconGovernanceTransfer(
              "0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537",
            ),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginRandomBeaconGovernanceTransfer(
            "0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537",
          )
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("when new governance is the zero address", () => {
        it("should revert", async () => {
          await expect(
            randomBeaconGovernance
              .connect(governance)
              .beginRandomBeaconGovernanceTransfer(ethers.ZeroAddress),
          ).to.be.revertedWith(
            "New random beacon governance address cannot be zero",
          )
        })
      })

      it("should not transfer the governance", async () => {
        expect(await randomBeacon.governance()).to.be.equal(
          await randomBeaconGovernance.getAddress(),
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingRandomBeaconGovernanceTransferDelayTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit RandomBeaconGovernanceTransferStarted", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "RandomBeaconGovernanceTransferStarted",
          )
          .withArgs(
            "0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537",
            blockTimestamp,
          )
      })
    })
  })

  describe("finalizeRandomBeaconGovernanceTransfer", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeRandomBeaconGovernanceTransfer(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeRandomBeaconGovernanceTransfer(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      before(async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginRandomBeaconGovernanceTransfer(
            "0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537",
          )

        await helpers.time.increaseTime(governanceDelay - 60) // -1min
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeRandomBeaconGovernanceTransfer(),
        ).to.be.revertedWith("Governance delay has not elapsed")
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginRandomBeaconGovernanceTransfer(
              "0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537",
            )

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeRandomBeaconGovernanceTransfer()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should transfer random beacon governance", async () => {
          expect(await randomBeacon.governance()).to.be.equal(
            "0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537",
          )
        })

        it("should emit RandomBeaconGovernanceTransferred event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "RandomBeaconGovernanceTransferred",
            )
            .withArgs("0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537")
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingRandomBeaconGovernanceTransferDelayTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginRelayEntrySoftTimeoutUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginRelayEntrySoftTimeoutUpdate(1),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update value is zero", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .beginRelayEntrySoftTimeoutUpdate(0),
        ).to.be.revertedWith("Relay entry soft timeout must be > 0")
      })
    })

    context("when the update value is at least one", () => {
      it("should accept the value", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginRelayEntrySoftTimeoutUpdate(1)
        await randomBeaconGovernance
          .connect(governance)
          .beginRelayEntrySoftTimeoutUpdate(2)

        // works, did not revert

        await restoreSnapshot()
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginRelayEntrySoftTimeoutUpdate(1)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the relay entry soft timeout", async () => {
        const { relayEntrySoftTimeout } =
          await randomBeacon.relayEntryParameters()
        expect(relayEntrySoftTimeout).to.be.equal(params.relayEntrySoftTimeout)
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingRelayEntrySoftTimeoutUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the RelayEntrySoftTimeoutUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(randomBeaconGovernance, "RelayEntrySoftTimeoutUpdateStarted")
          .withArgs(1, blockTimestamp)
      })
    })
  })

  describe("finalizeRelayEntrySoftTimeoutUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeRelayEntrySoftTimeoutUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntrySoftTimeoutUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginRelayEntrySoftTimeoutUpdate(1)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntrySoftTimeoutUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginRelayEntrySoftTimeoutUpdate(1)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntrySoftTimeoutUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the relay entry soft timeout", async () => {
          const { relayEntrySoftTimeout } =
            await randomBeacon.relayEntryParameters()
          expect(relayEntrySoftTimeout).to.be.equal(1)
        })

        it("should emit RelayEntrySoftTimeoutUpdated event", async () => {
          await expect(tx)
            .to.emit(randomBeaconGovernance, "RelayEntrySoftTimeoutUpdated")
            .withArgs(1)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingRelayEntrySoftTimeoutUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginRelayEntryHardTimeoutUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginRelayEntryHardTimeoutUpdate(123),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginRelayEntryHardTimeoutUpdate(123)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the relay entry hard timeout", async () => {
        const { relayEntryHardTimeout } =
          await randomBeacon.relayEntryParameters()
        expect(relayEntryHardTimeout).to.be.equal(params.relayEntryHardTimeout)
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingRelayEntryHardTimeoutUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the RelayEntryHardTimeoutUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(randomBeaconGovernance, "RelayEntryHardTimeoutUpdateStarted")
          .withArgs(123, blockTimestamp)
      })
    })
  })

  describe("finalizeRelayEntryHardTimeoutUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeRelayEntryHardTimeoutUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntryHardTimeoutUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginRelayEntryHardTimeoutUpdate(123)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntryHardTimeoutUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginRelayEntryHardTimeoutUpdate(123)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntryHardTimeoutUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the relay entry hard timeout", async () => {
          const { relayEntryHardTimeout } =
            await randomBeacon.relayEntryParameters()
          expect(relayEntryHardTimeout).to.be.equal(123)
        })

        it("should emit RelayEntryHardTimeoutUpdated event", async () => {
          await expect(tx)
            .to.emit(randomBeaconGovernance, "RelayEntryHardTimeoutUpdated")
            .withArgs(123)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingRelayEntryHardTimeoutUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginCallbackGasLimitUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginCallbackGasLimitUpdate(123),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update value is zero", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .beginCallbackGasLimitUpdate(0),
        ).to.be.revertedWith("Callback gas limit must be > 0 and <= 1000000")
      })
    })

    context("when the update value is at least one", () => {
      it("should accept the value", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginCallbackGasLimitUpdate(1)
        await randomBeaconGovernance
          .connect(governance)
          .beginCallbackGasLimitUpdate(2)

        // works, did not revert

        await restoreSnapshot()
      })
    })

    context("when the update value is more than one million", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .beginCallbackGasLimitUpdate(1000001),
        ).to.be.revertedWith("Callback gas limit must be > 0 and <= 1000000")
      })
    })

    context("when the update value is one million", () => {
      it("should accept the value", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginCallbackGasLimitUpdate(1000000)

        // works, did not revert

        await restoreSnapshot()
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginCallbackGasLimitUpdate(123)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the callback gas limit", async () => {
        const { callbackGasLimit } = await randomBeacon.relayEntryParameters()
        expect(callbackGasLimit).to.be.equal(params.callbackGasLimit)
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingCallbackGasLimitUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the CallbackGasLimitUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(randomBeaconGovernance, "CallbackGasLimitUpdateStarted")
          .withArgs(123, blockTimestamp)
      })
    })
  })

  describe("finalizeCallbackGasLimitUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeCallbackGasLimitUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeCallbackGasLimitUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginCallbackGasLimitUpdate(123)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeCallbackGasLimitUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginCallbackGasLimitUpdate(123)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeCallbackGasLimitUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the callback gas limit", async () => {
          const { callbackGasLimit } = await randomBeacon.relayEntryParameters()
          expect(callbackGasLimit).to.be.equal(123)
        })

        it("should emit CallbackGasLimitUpdated event", async () => {
          await expect(tx)
            .to.emit(randomBeaconGovernance, "CallbackGasLimitUpdated")
            .withArgs(123)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingCallbackGasLimitUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginGroupCreationFrequencyUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginGroupCreationFrequencyUpdate(1),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update value is zero", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .beginGroupCreationFrequencyUpdate(0),
        ).to.be.revertedWith("Group creation frequency must be > 0")
      })
    })

    context("when the update value is at least one", () => {
      it("should accept the value", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginGroupCreationFrequencyUpdate(1)
        await randomBeaconGovernance
          .connect(governance)
          .beginGroupCreationFrequencyUpdate(2)

        // works, did not revert

        await restoreSnapshot()
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginGroupCreationFrequencyUpdate(1)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the group creation frequency timeout", async () => {
        const { groupCreationFrequency } =
          await randomBeacon.groupCreationParameters()
        expect(groupCreationFrequency).to.be.equal(
          params.groupCreationFrequency,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingGroupCreationFrequencyUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the GroupCreationFrequencyUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "GroupCreationFrequencyUpdateStarted",
          )
          .withArgs(1, blockTimestamp)
      })
    })
  })

  describe("finalizeGroupCreationFrequencyUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeGroupCreationFrequencyUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeGroupCreationFrequencyUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginGroupCreationFrequencyUpdate(1)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeGroupCreationFrequencyUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginGroupCreationFrequencyUpdate(1)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeGroupCreationFrequencyUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the group creation frequency", async () => {
          const { groupCreationFrequency } =
            await randomBeacon.groupCreationParameters()
          expect(groupCreationFrequency).to.be.equal(1)
        })

        it("should emit GroupCreationFrequencyUpdated event", async () => {
          await expect(tx)
            .to.emit(randomBeaconGovernance, "GroupCreationFrequencyUpdated")
            .withArgs(1)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingGroupCreationFrequencyUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginGroupLifetimeUpdate", () => {
    const newGroupLifetime = params.groupLifetime + 1

    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginGroupLifetimeUpdate(newGroupLifetime),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update value is zero", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .beginGroupLifetimeUpdate(0),
        ).to.be.revertedWith("Group lifetime must be greater than 0")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginGroupLifetimeUpdate(newGroupLifetime)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the group lifetime", async () => {
        const { groupLifetime } = await randomBeacon.groupCreationParameters()
        expect(groupLifetime).to.be.equal(params.groupLifetime)
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingGroupLifetimeUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the GroupLifetimeUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(randomBeaconGovernance, "GroupLifetimeUpdateStarted")
          .withArgs(newGroupLifetime, blockTimestamp)
      })
    })
  })

  describe("finalizeGroupLifetimeUpdate", () => {
    const newGroupLifetime = params.groupLifetime + 1

    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeGroupLifetimeUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeGroupLifetimeUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginGroupLifetimeUpdate(newGroupLifetime)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeGroupLifetimeUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginGroupLifetimeUpdate(newGroupLifetime)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeGroupLifetimeUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the group lifetime", async () => {
          const { groupLifetime } = await randomBeacon.groupCreationParameters()
          expect(groupLifetime).to.be.equal(newGroupLifetime)
        })

        it("should emit GroupLifetimeUpdated event", async () => {
          await expect(tx)
            .to.emit(randomBeaconGovernance, "GroupLifetimeUpdated")
            .withArgs(newGroupLifetime)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingGroupLifetimeUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginDkgResultChallengePeriodLengthUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginDkgResultChallengePeriodLengthUpdate(11),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update value is less than 10", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .beginDkgResultChallengePeriodLengthUpdate(9),
        ).to.be.revertedWith("DKG result challenge period length must be >= 10")
      })
    })

    context("when the update value is at least 10", () => {
      it("should accept the value", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultChallengePeriodLengthUpdate(10)
        await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultChallengePeriodLengthUpdate(11)

        // works, did not revert

        await restoreSnapshot()
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultChallengePeriodLengthUpdate(11)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG result challenge period length", async () => {
        const { dkgResultChallengePeriodLength } =
          await randomBeacon.groupCreationParameters()
        expect(dkgResultChallengePeriodLength).to.be.equal(
          params.dkgResultChallengePeriodLength,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingDkgResultChallengePeriodLengthUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the DkgResultChallengePeriodLengthUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "DkgResultChallengePeriodLengthUpdateStarted",
          )
          .withArgs(11, blockTimestamp)
      })
    })
  })

  describe("finalizeDkgResultChallengePeriodLengthUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeDkgResultChallengePeriodLengthUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultChallengePeriodLengthUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultChallengePeriodLengthUpdate(11)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultChallengePeriodLengthUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginDkgResultChallengePeriodLengthUpdate(11)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultChallengePeriodLengthUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG result challenge period length", async () => {
          const { dkgResultChallengePeriodLength } =
            await randomBeacon.groupCreationParameters()
          expect(dkgResultChallengePeriodLength).to.be.equal(11)
        })

        it("should emit DkgResultChallengePeriodLengthUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "DkgResultChallengePeriodLengthUpdated",
            )
            .withArgs(11)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingDkgResultChallengePeriodLengthUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginDkgResultChallengeExtraGasUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginDkgResultChallengeExtraGasUpdate(1337),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultChallengeExtraGasUpdate(1337)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG result challenge extra gas", async () => {
        const { dkgResultChallengeExtraGas } =
          await randomBeacon.groupCreationParameters()
        expect(dkgResultChallengeExtraGas).to.be.equal(
          params.dkgResultChallengeExtraGas,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingDkgResultChallengeExtraGasUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the DkgResultChallengeExtraGasUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "DkgResultChallengeExtraGasUpdateStarted",
          )
          .withArgs(1337, blockTimestamp)
      })
    })
  })

  describe("finalizeDkgResultChallengeExtraGasUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeDkgResultChallengeExtraGasUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultChallengeExtraGasUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultChallengeExtraGasUpdate(1337)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultChallengeExtraGasUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginDkgResultChallengeExtraGasUpdate(1337)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultChallengeExtraGasUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG result challenge extra gas", async () => {
          const { dkgResultChallengeExtraGas } =
            await randomBeacon.groupCreationParameters()
          expect(dkgResultChallengeExtraGas).to.be.equal(1337)
        })

        it("should emit DkgResultChallengeExtraGasUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "DkgResultChallengeExtraGasUpdated",
            )
            .withArgs(1337)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingDkgResultChallengeExtraGasUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginDkgResultSubmissionTimeoutUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginDkgResultSubmissionTimeoutUpdate(1),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update value is zero", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .beginDkgResultSubmissionTimeoutUpdate(0),
        ).to.be.revertedWith("DKG result submission timeout must be > 0")
      })
    })

    context("when the update value is at least one", () => {
      it("should accept the value", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultSubmissionTimeoutUpdate(1)
        await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultSubmissionTimeoutUpdate(2)

        // works, did not revert

        await restoreSnapshot()
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultSubmissionTimeoutUpdate(1)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG result submission timeout", async () => {
        const { dkgResultSubmissionTimeout } =
          await randomBeacon.groupCreationParameters()
        expect(dkgResultSubmissionTimeout).to.be.equal(
          params.dkgResultSubmissionTimeout,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingDkgResultSubmissionTimeoutUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the DkgResultSubmissionTimeoutUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "DkgResultSubmissionTimeoutUpdateStarted",
          )
          .withArgs(1, blockTimestamp)
      })
    })
  })

  describe("finalizeDkgResultSubmissionTimeoutUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeDkgResultSubmissionTimeoutUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultSubmissionTimeoutUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultSubmissionTimeoutUpdate(1)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultSubmissionTimeoutUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        const newValue = 234
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginDkgResultSubmissionTimeoutUpdate(newValue)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultSubmissionTimeoutUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG result submission timeout", async () => {
          const { dkgResultSubmissionTimeout } =
            await randomBeacon.groupCreationParameters()
          expect(dkgResultSubmissionTimeout).to.be.equal(newValue)
        })

        it("should emit DkgResultSubmissionTimeoutUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "DkgResultSubmissionTimeoutUpdated",
            )
            .withArgs(newValue)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingDkgResultSubmissionTimeoutUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginDkgSubmitterPrecedencePeriodLengthUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginDkgSubmitterPrecedencePeriodLengthUpdate(1),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update value is zero", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .beginDkgSubmitterPrecedencePeriodLengthUpdate(0),
        ).to.be.revertedWith(
          "DKG submitter precedence period length must be > 0",
        )
      })
    })

    context("when the update value is at least one", () => {
      it("should accept the value", async () => {
        await createSnapshot()

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .beginDkgSubmitterPrecedencePeriodLengthUpdate(1),
        ).not.to.be.reverted

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .beginDkgSubmitterPrecedencePeriodLengthUpdate(2),
        ).not.to.be.reverted

        await restoreSnapshot()
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginDkgSubmitterPrecedencePeriodLengthUpdate(1)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG submitter precedence period length", async () => {
        const { dkgSubmitterPrecedencePeriodLength } =
          await randomBeacon.groupCreationParameters()
        expect(dkgSubmitterPrecedencePeriodLength).to.be.equal(
          params.dkgSubmitterPrecedencePeriodLength,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingDkgSubmitterPrecedencePeriodLengthUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the DkgSubmitterPrecedencePeriodLengthUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "DkgSubmitterPrecedencePeriodLengthUpdateStarted",
          )
          .withArgs(1, blockTimestamp)
      })
    })
  })

  describe("finalizeDkgSubmitterPrecedencePeriodLengthUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeDkgSubmitterPrecedencePeriodLengthUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgSubmitterPrecedencePeriodLengthUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginDkgSubmitterPrecedencePeriodLengthUpdate(1)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgSubmitterPrecedencePeriodLengthUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginDkgSubmitterPrecedencePeriodLengthUpdate(1)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeDkgSubmitterPrecedencePeriodLengthUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG submitter precedence period length", async () => {
          const { dkgSubmitterPrecedencePeriodLength } =
            await randomBeacon.groupCreationParameters()
          expect(dkgSubmitterPrecedencePeriodLength).to.be.equal(1)
        })

        it("should emit DkgSubmitterPrecedencePeriodLengthUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "DkgSubmitterPrecedencePeriodLengthUpdated",
            )
            .withArgs(1)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingDkgSubmitterPrecedencePeriodLengthUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginRelayEntrySubmissionFailureSlashingAmountUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginRelayEntrySubmissionFailureSlashingAmountUpdate(123),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginRelayEntrySubmissionFailureSlashingAmountUpdate(123)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the relay entry submission failure slashing amount", async () => {
        const { relayEntrySubmissionFailureSlashingAmount } =
          await randomBeacon.slashingParameters()
        expect(relayEntrySubmissionFailureSlashingAmount).to.be.equal(
          params.relayEntrySubmissionFailureSlashingAmount,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingRelayEntrySubmissionFailureSlashingAmountUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the RelayEntrySubmissionFailureSlashingAmountUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "RelayEntrySubmissionFailureSlashingAmountUpdateStarted",
          )
          .withArgs(123, blockTimestamp)
      })
    })
  })

  describe("finalizeRelayEntrySubmissionFailureSlashingAmountUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeRelayEntrySubmissionFailureSlashingAmountUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntrySubmissionFailureSlashingAmountUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginRelayEntrySubmissionFailureSlashingAmountUpdate(123)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntrySubmissionFailureSlashingAmountUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginRelayEntrySubmissionFailureSlashingAmountUpdate(123)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntrySubmissionFailureSlashingAmountUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the relay entry submission failure slashing amount", async () => {
          const { relayEntrySubmissionFailureSlashingAmount } =
            await randomBeacon.slashingParameters()
          expect(relayEntrySubmissionFailureSlashingAmount).to.be.equal(123)
        })

        it("should emit RelayEntrySubmissionFailureSlashingAmountUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "RelayEntrySubmissionFailureSlashingAmountUpdated",
            )
            .withArgs(123)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingRelayEntrySubmissionFailureSlashingAmountUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginUnauthorizedSigningSlashingAmountUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginUnauthorizedSigningSlashingAmountUpdate(123),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginUnauthorizedSigningSlashingAmountUpdate(123)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the unauthorized signing slashing amount", async () => {
        const { unauthorizedSigningSlashingAmount } =
          await randomBeacon.slashingParameters()
        expect(unauthorizedSigningSlashingAmount).to.be.equal(
          params.unauthorizedSigningSlashingAmount,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingUnauthorizedSigningSlashingAmountUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the UnauthorizedSigningSlashingAmountUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "UnauthorizedSigningSlashingAmountUpdateStarted",
          )
          .withArgs(123, blockTimestamp)
      })
    })
  })

  describe("finalizeUnauthorizedSigningSlashingAmountUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeUnauthorizedSigningSlashingAmountUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeUnauthorizedSigningSlashingAmountUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginUnauthorizedSigningSlashingAmountUpdate(123)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeUnauthorizedSigningSlashingAmountUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginUnauthorizedSigningSlashingAmountUpdate(123)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeUnauthorizedSigningSlashingAmountUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the unauthorized signing slashing amount", async () => {
          const { unauthorizedSigningSlashingAmount } =
            await randomBeacon.slashingParameters()
          expect(unauthorizedSigningSlashingAmount).to.be.equal(123)
        })

        it("should emit UnauthorizedSigningSlashingAmountUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "UnauthorizedSigningSlashingAmountUpdated",
            )
            .withArgs(123)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingUnauthorizedSigningSlashingAmountUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginMaliciousDkgResultSlashingAmountUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginMaliciousDkgResultSlashingAmountUpdate(123),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginMaliciousDkgResultSlashingAmountUpdate(123)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the malicious DKG result slashing amount", async () => {
        const { maliciousDkgResultSlashingAmount } =
          await randomBeacon.slashingParameters()
        expect(maliciousDkgResultSlashingAmount).to.be.equal(
          params.maliciousDkgResultSlashingAmount,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingMaliciousDkgResultSlashingAmountUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the MaliciousDkgResultSlashingAmountUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "MaliciousDkgResultSlashingAmountUpdateStarted",
          )
          .withArgs(123, blockTimestamp)
      })
    })
  })

  describe("finalizeMaliciousDkgResultSlashingAmountUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeMaliciousDkgResultSlashingAmountUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeMaliciousDkgResultSlashingAmountUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginMaliciousDkgResultSlashingAmountUpdate(123)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeMaliciousDkgResultSlashingAmountUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginMaliciousDkgResultSlashingAmountUpdate(123)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeMaliciousDkgResultSlashingAmountUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the malicious DKG result slashing amount", async () => {
          const { maliciousDkgResultSlashingAmount } =
            await randomBeacon.slashingParameters()
          expect(maliciousDkgResultSlashingAmount).to.be.equal(123)
        })

        it("should emit MaliciousDkgResultSlashingAmountUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "MaliciousDkgResultSlashingAmountUpdated",
            )
            .withArgs(123)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingMaliciousDkgResultSlashingAmountUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginSortitionPoolRewardsBanDurationUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginSortitionPoolRewardsBanDurationUpdate(123),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginSortitionPoolRewardsBanDurationUpdate(123)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the sortition pool rewards ban duration", async () => {
        const { sortitionPoolRewardsBanDuration } =
          await randomBeacon.rewardParameters()
        expect(sortitionPoolRewardsBanDuration).to.be.equal(
          params.sortitionPoolRewardsBanDuration,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingSortitionPoolRewardsBanDurationUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the SortitionPoolRewardsBanDurationUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "SortitionPoolRewardsBanDurationUpdateStarted",
          )
          .withArgs(123, blockTimestamp)
      })
    })
  })

  describe("finalizeSortitionPoolRewardsBanDurationUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeSortitionPoolRewardsBanDurationUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeSortitionPoolRewardsBanDurationUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginSortitionPoolRewardsBanDurationUpdate(123)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeSortitionPoolRewardsBanDurationUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginSortitionPoolRewardsBanDurationUpdate(123)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeSortitionPoolRewardsBanDurationUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the sortition pool rewards ban duration", async () => {
          const { sortitionPoolRewardsBanDuration } =
            await randomBeacon.rewardParameters()
          expect(sortitionPoolRewardsBanDuration).to.be.equal(123)
        })

        it("should emit SortitionPoolRewardsBanDurationUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "SortitionPoolRewardsBanDurationUpdated",
            )
            .withArgs(123)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingSortitionPoolRewardsBanDurationUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginUnauthorizedSigningNotificationRewardMultiplierUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginUnauthorizedSigningNotificationRewardMultiplierUpdate(100),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when called with value >100", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .beginUnauthorizedSigningNotificationRewardMultiplierUpdate(101),
        ).to.be.revertedWith("Maximum value is 100")
      })
    })

    context("when the caller is the owner and value is correct", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginUnauthorizedSigningNotificationRewardMultiplierUpdate(100)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the unauthorized signing notification reward multiplier", async () => {
        const { unauthorizedSigningNotificationRewardMultiplier } =
          await randomBeacon.rewardParameters()
        expect(unauthorizedSigningNotificationRewardMultiplier).to.be.equal(
          params.unauthorizedSigningNotificationRewardMultiplier,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingUnauthorizedSigningNotificationRewardMultiplierUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the UnauthorizedSigningNotificationRewardMultiplierUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "UnauthorizedSigningNotificationRewardMultiplierUpdateStarted",
          )
          .withArgs(100, blockTimestamp)
      })
    })
  })

  describe("finalizeUnauthorizedSigningNotificationRewardMultiplierUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeUnauthorizedSigningNotificationRewardMultiplierUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeUnauthorizedSigningNotificationRewardMultiplierUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginUnauthorizedSigningNotificationRewardMultiplierUpdate(100)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeUnauthorizedSigningNotificationRewardMultiplierUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginUnauthorizedSigningNotificationRewardMultiplierUpdate(100)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeUnauthorizedSigningNotificationRewardMultiplierUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the unauthorized signing notification reward multiplier", async () => {
          const { unauthorizedSigningNotificationRewardMultiplier } =
            await randomBeacon.rewardParameters()
          expect(unauthorizedSigningNotificationRewardMultiplier).to.be.equal(
            100,
          )
        })

        it("should emit UnauthorizedSigningNotificationRewardMultiplierUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "UnauthorizedSigningNotificationRewardMultiplierUpdated",
            )
            .withArgs(100)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingUnauthorizedSigningNotificationRewardMultiplierUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginRelayEntryTimeoutNotificationRewardMultiplierUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginRelayEntryTimeoutNotificationRewardMultiplierUpdate(100),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when called with value >100", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .beginRelayEntryTimeoutNotificationRewardMultiplierUpdate(101),
        ).to.be.revertedWith("Maximum value is 100")
      })
    })

    context("when the caller is the owner and value is correct", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginRelayEntryTimeoutNotificationRewardMultiplierUpdate(100)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the relay entry timeout notification reward multiplier", async () => {
        const { relayEntryTimeoutNotificationRewardMultiplier } =
          await randomBeacon.rewardParameters()
        expect(relayEntryTimeoutNotificationRewardMultiplier).to.be.equal(
          params.relayEntryTimeoutNotificationRewardMultiplier,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingRelayEntryTimeoutNotificationRewardMultiplierUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the RelayEntryTimeoutNotificationRewardMultiplierUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "RelayEntryTimeoutNotificationRewardMultiplierUpdateStarted",
          )
          .withArgs(100, blockTimestamp)
      })
    })
  })

  describe("finalizeRelayEntryTimeoutNotificationRewardMultiplierUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeRelayEntryTimeoutNotificationRewardMultiplierUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntryTimeoutNotificationRewardMultiplierUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginRelayEntryTimeoutNotificationRewardMultiplierUpdate(100)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntryTimeoutNotificationRewardMultiplierUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginRelayEntryTimeoutNotificationRewardMultiplierUpdate(100)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntryTimeoutNotificationRewardMultiplierUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the relay entry timeout notification reward multiplier", async () => {
          const { relayEntryTimeoutNotificationRewardMultiplier } =
            await randomBeacon.rewardParameters()

          expect(relayEntryTimeoutNotificationRewardMultiplier).to.be.equal(100)
        })

        it("should emit RelayEntryTimeoutNotificationRewardMultiplierUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "RelayEntryTimeoutNotificationRewardMultiplierUpdated",
            )
            .withArgs(100)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingRelayEntryTimeoutNotificationRewardMultiplierUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginMinimumAuthorizationUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginMinimumAuthorizationUpdate(123),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginMinimumAuthorizationUpdate(123)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the minimum authorization amount", async () => {
        expect(await randomBeacon.minimumAuthorization()).to.be.equal(
          params.minimumAuthorization,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingMimimumAuthorizationUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the MinimumAuthorizationUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(randomBeaconGovernance, "MinimumAuthorizationUpdateStarted")
          .withArgs(123, blockTimestamp)
      })
    })
  })

  describe("finalizeMinimumAuthorizationUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeMinimumAuthorizationUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeMinimumAuthorizationUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginMinimumAuthorizationUpdate(123)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeMinimumAuthorizationUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginMinimumAuthorizationUpdate(123)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeMinimumAuthorizationUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the minimum authorization amount", async () => {
          expect(await randomBeacon.minimumAuthorization()).to.be.equal(123)
        })

        it("should emit MinimumAuthorizationUpdated event", async () => {
          await expect(tx)
            .to.emit(randomBeaconGovernance, "MinimumAuthorizationUpdated")
            .withArgs(123)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingMimimumAuthorizationUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginAuthorizationDecreaseDelayUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginAuthorizationDecreaseDelayUpdate(123),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginAuthorizationDecreaseDelayUpdate(123)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the authorization decrease delay", async () => {
        const { authorizationDecreaseDelay } =
          await randomBeacon.authorizationParameters()
        expect(authorizationDecreaseDelay).to.be.equal(
          params.authorizationDecreaseDelay,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingAuthorizationDecreaseDelayUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the AuthorizationDecreaseDelayUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "AuthorizationDecreaseDelayUpdateStarted",
          )
          .withArgs(123, blockTimestamp)
      })
    })
  })

  describe("finalizeAuthorizationDecreaseDelayUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeAuthorizationDecreaseDelayUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeAuthorizationDecreaseDelayUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginAuthorizationDecreaseDelayUpdate(123)
        await helpers.time.increaseTime(governanceDelay - 60) // -1min
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeAuthorizationDecreaseDelayUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginAuthorizationDecreaseDelayUpdate(123)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeAuthorizationDecreaseDelayUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the authorization decrease delay", async () => {
          const { authorizationDecreaseDelay } =
            await randomBeacon.authorizationParameters()
          expect(authorizationDecreaseDelay).to.be.equal(123)
        })

        it("should emit AuthorizationDecreaseDelayUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "AuthorizationDecreaseDelayUpdated",
            )
            .withArgs(123)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingAuthorizationDecreaseDelayUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginAuthorizationDecreaseChangePeriodUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginAuthorizationDecreaseChangePeriodUpdate(123),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginAuthorizationDecreaseChangePeriodUpdate(123)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the authorization decrease change period", async () => {
        const { authorizationDecreaseChangePeriod } =
          await randomBeacon.authorizationParameters()
        expect(authorizationDecreaseChangePeriod).to.be.equal(
          params.authorizationDecreaseChangePeriod,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingAuthorizationDecreaseChangePeriodUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the AuthorizationDecreaseChangePeriodUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "AuthorizationDecreaseChangePeriodUpdateStarted",
          )
          .withArgs(123, blockTimestamp)
      })
    })
  })

  describe("finalizeAuthorizationDecreaseChangePeriodUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeAuthorizationDecreaseChangePeriodUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeAuthorizationDecreaseChangePeriodUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginAuthorizationDecreaseChangePeriodUpdate(123)
        await helpers.time.increaseTime(governanceDelay - 60) // -1min
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeAuthorizationDecreaseChangePeriodUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginAuthorizationDecreaseChangePeriodUpdate(123)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeAuthorizationDecreaseChangePeriodUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the authorization decrease change period", async () => {
          const { authorizationDecreaseChangePeriod } =
            await randomBeacon.authorizationParameters()
          expect(authorizationDecreaseChangePeriod).to.be.equal(123)
        })

        it("should emit AuthorizationDecreaseChangePeriodUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "AuthorizationDecreaseChangePeriodUpdated",
            )
            .withArgs(123)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingAuthorizationDecreaseChangePeriodUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("setRequesterAuthorization", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .setRequesterAuthorization(thirdPartyContract.address, true),
        ).to.be.revertedWith("Ownable: caller is not the owner")

        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .setRequesterAuthorization(thirdPartyContract.address, false),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      it("should update requester authorization", async () => {
        let isAuthorized = await randomBeacon.authorizedRequesters(
          thirdPartyContract.address,
        )
        await expect(isAuthorized).to.be.false

        await randomBeaconGovernance
          .connect(governance)
          .setRequesterAuthorization(thirdPartyContract.address, true)

        isAuthorized = await randomBeacon.authorizedRequesters(
          thirdPartyContract.address,
        )
        await expect(isAuthorized).to.be.true

        await randomBeaconGovernance
          .connect(governance)
          .setRequesterAuthorization(thirdPartyContract.address, false)

        isAuthorized = await randomBeacon.authorizedRequesters(
          thirdPartyContract.address,
        )
        await expect(isAuthorized).to.be.false
      })
    })
  })

  describe("beginDkgMaliciousResultNotificationRewardMultiplierUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginDkgMaliciousResultNotificationRewardMultiplierUpdate(100),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when called with value >100", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .beginDkgMaliciousResultNotificationRewardMultiplierUpdate(101),
        ).to.be.revertedWith("Maximum value is 100")
      })
    })

    context("when the caller is the owner and value is correct", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginDkgMaliciousResultNotificationRewardMultiplierUpdate(100)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG malicious result notification reward multiplier", async () => {
        const { dkgMaliciousResultNotificationRewardMultiplier } =
          await randomBeacon.rewardParameters()

        expect(dkgMaliciousResultNotificationRewardMultiplier).to.be.equal(
          params.dkgMaliciousResultNotificationRewardMultiplier,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingDkgMaliciousResultNotificationRewardMultiplierUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the DkgMaliciousResultNotificationRewardMultiplierUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "DkgMaliciousResultNotificationRewardMultiplierUpdateStarted",
          )
          .withArgs(100, blockTimestamp)
      })
    })
  })

  describe("finalizeDkgMaliciousResultNotificationRewardMultiplierUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeDkgMaliciousResultNotificationRewardMultiplierUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgMaliciousResultNotificationRewardMultiplierUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginDkgMaliciousResultNotificationRewardMultiplierUpdate(100)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgMaliciousResultNotificationRewardMultiplierUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginDkgMaliciousResultNotificationRewardMultiplierUpdate(100)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeDkgMaliciousResultNotificationRewardMultiplierUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG malicious result notification reward multiplier", async () => {
          const { dkgMaliciousResultNotificationRewardMultiplier } =
            await randomBeacon.rewardParameters()
          expect(dkgMaliciousResultNotificationRewardMultiplier).to.be.equal(
            100,
          )
        })

        it("should emit DkgMaliciousResultNotificationRewardMultiplierUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "DkgMaliciousResultNotificationRewardMultiplierUpdated",
            )
            .withArgs(100)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingDkgMaliciousResultNotificationRewardMultiplierUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginDkgResultSubmissionGasUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginDkgResultSubmissionGasUpdate(100),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultSubmissionGasUpdate(1337)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update DKG result submission gas", async () => {
        const { dkgResultSubmissionGas } = await randomBeacon.gasParameters()
        expect(dkgResultSubmissionGas).to.be.equal(
          params.dkgResultSubmissionGas,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingDkgResultSubmissionGasUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit DkgResultSubmissionGasUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "DkgResultSubmissionGasUpdateStarted",
          )
          .withArgs(1337, blockTimestamp)
      })
    })
  })

  context("finalizeDkgResultSubmissionGasUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeDkgResultSubmissionGasUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultSubmissionGasUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultSubmissionGasUpdate(1337)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultSubmissionGasUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginDkgResultSubmissionGasUpdate(1337)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultSubmissionGasUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG result submission gas", async () => {
          const { dkgResultSubmissionGas } = await randomBeacon.gasParameters()
          expect(dkgResultSubmissionGas).to.be.equal(1337)
        })

        it("should emit DkgResultSubmissionGasUpdated event", async () => {
          await expect(tx)
            .to.emit(randomBeaconGovernance, "DkgResultSubmissionGasUpdated")
            .withArgs(1337)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingDkgResultSubmissionGasUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginDkgResultApprovalGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginDkgResultApprovalGasOffsetUpdate(1337),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultApprovalGasOffsetUpdate(1337)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG result approval gas offset", async () => {
        const { dkgResultApprovalGasOffset } =
          await randomBeacon.gasParameters()
        expect(dkgResultApprovalGasOffset).to.be.equal(
          params.dkgResultApprovalGasOffset,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingDkgResultApprovalGasOffsetUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the DkgResultApprovalGasOffsetUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "DkgResultApprovalGasOffsetUpdateStarted",
          )
          .withArgs(1337, blockTimestamp)
      })
    })
  })

  describe("finalizeDkgResultApprovalGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeDkgResultApprovalGasOffsetUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultApprovalGasOffsetUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginDkgResultApprovalGasOffsetUpdate(7331)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultApprovalGasOffsetUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginDkgResultApprovalGasOffsetUpdate(7331)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeDkgResultApprovalGasOffsetUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG result approval gas offset", async () => {
          const { dkgResultApprovalGasOffset } =
            await randomBeacon.gasParameters()
          expect(dkgResultApprovalGasOffset).to.be.equal(7331)
        })

        it("should emit DkgResultApprovalGasOffsetUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "DkgResultApprovalGasOffsetUpdated",
            )
            .withArgs(7331)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingDkgResultApprovalGasOffsetUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginNotifyOperatorInactivityGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginNotifyOperatorInactivityGasOffsetUpdate(100),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginNotifyOperatorInactivityGasOffsetUpdate(100)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the notify operator inactivity gas offset", async () => {
        const { notifyOperatorInactivityGasOffset } =
          await randomBeacon.gasParameters()
        expect(notifyOperatorInactivityGasOffset).to.be.equal(
          params.notifyOperatorInactivityGasOffset,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingNotifyOperatorInactivityGasOffsetUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the NotifyOperatorInactivityGasOffsetUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "NotifyOperatorInactivityGasOffsetUpdateStarted",
          )
          .withArgs(100, blockTimestamp)
      })
    })
  })

  describe("finalizeNotifyOperatorInactivityGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeNotifyOperatorInactivityGasOffsetUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeNotifyOperatorInactivityGasOffsetUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginNotifyOperatorInactivityGasOffsetUpdate(100)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeNotifyOperatorInactivityGasOffsetUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginNotifyOperatorInactivityGasOffsetUpdate(100)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeNotifyOperatorInactivityGasOffsetUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the notify operator inactivity gas offset", async () => {
          const { notifyOperatorInactivityGasOffset } =
            await randomBeacon.gasParameters()
          expect(notifyOperatorInactivityGasOffset).to.be.equal(100)
        })

        it("should emit NotifyOperatorInactivityGasOffsetUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "NotifyOperatorInactivityGasOffsetUpdated",
            )
            .withArgs(100)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingNotifyOperatorInactivityGasOffsetUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("beginRelayEntrySubmissionGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .beginRelayEntrySubmissionGasOffsetUpdate(997),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransactionResponse

      before(async () => {
        await createSnapshot()

        tx = await randomBeaconGovernance
          .connect(governance)
          .beginRelayEntrySubmissionGasOffsetUpdate(997)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the relay entry submission gas offset", async () => {
        const { relayEntrySubmissionGasOffset } =
          await randomBeacon.gasParameters()
        expect(relayEntrySubmissionGasOffset).to.be.equal(
          params.relayEntrySubmissionGasOffset,
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await randomBeaconGovernance.getRemainingRelayEntrySubmissionGasOffsetUpdateTime(),
        ).to.be.equal(governanceDelay)
      })

      it("should emit the RelayEntrySubmissionGasOffsetUpdateStarted event", async () => {
        const blockTimestamp = requireResult(
          await ethers.provider.getBlock(
            requireResult(await tx.wait()).blockNumber,
          ),
        ).timestamp
        await expect(tx)
          .to.emit(
            randomBeaconGovernance,
            "RelayEntrySubmissionGasOffsetUpdateStarted",
          )
          .withArgs(997, blockTimestamp)
      })
    })
  })

  describe("finalizeRelayEntrySubmissionGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .finalizeRelayEntrySubmissionGasOffsetUpdate(),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntrySubmissionGasOffsetUpdate(),
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await randomBeaconGovernance
          .connect(governance)
          .beginRelayEntrySubmissionGasOffsetUpdate(997)

        await helpers.time.increaseTime(governanceDelay - 60) // -1min

        await expect(
          randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntrySubmissionGasOffsetUpdate(),
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransactionResponse

        before(async () => {
          await createSnapshot()

          await randomBeaconGovernance
            .connect(governance)
            .beginRelayEntrySubmissionGasOffsetUpdate(997)

          await helpers.time.increaseTime(governanceDelay)

          tx = await randomBeaconGovernance
            .connect(governance)
            .finalizeRelayEntrySubmissionGasOffsetUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the relay entry submission gas offset", async () => {
          const { relayEntrySubmissionGasOffset } =
            await randomBeacon.gasParameters()
          expect(relayEntrySubmissionGasOffset).to.be.equal(997)
        })

        it("should emit RelayEntrySubmissionGasOffsetUpdated event", async () => {
          await expect(tx)
            .to.emit(
              randomBeaconGovernance,
              "RelayEntrySubmissionGasOffsetUpdated",
            )
            .withArgs(997)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            randomBeaconGovernance.getRemainingRelayEntrySubmissionGasOffsetUpdateTime(),
          ).to.be.revertedWith("Change not initiated")
        })
      },
    )
  })

  describe("withdrawIneligibleRewards", () => {
    context("when caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          randomBeaconGovernance
            .connect(thirdParty)
            .withdrawIneligibleRewards(thirdParty.address),
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    // The actual functionality is tested in RandomBeacon.Rewards.test.ts
  })
})

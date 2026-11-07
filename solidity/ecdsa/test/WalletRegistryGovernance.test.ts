import { deployments, ethers, helpers } from "hardhat"
import { expect } from "chai"

import { constants, params, updateWalletRegistryParams } from "./fixtures"

import type { ContractTransaction } from "ethers"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type {
  WalletRegistry,
  WalletRegistryStub,
  WalletRegistryGovernance,
} from "../typechain"

const { createSnapshot, restoreSnapshot } = helpers.snapshot
const { to1e18 } = helpers.number

const fixture = deployments.createFixture(async () => {
  // Full fixture: tagged-only runs skip external @threshold-network deploys (e.g. T).
  await deployments.fixture()

  const walletRegistry: WalletRegistryStub & WalletRegistry =
    await helpers.contracts.getContract("WalletRegistry")
  const walletRegistryGovernance: WalletRegistryGovernance =
    await helpers.contracts.getContract("WalletRegistryGovernance")

  const { governance } = await helpers.signers.getNamedSigners()

  const [thirdParty] = await helpers.signers.getUnnamedSigners()

  await updateWalletRegistryParams(walletRegistryGovernance, governance)

  return {
    walletRegistry,
    walletRegistryGovernance,
    governance,
    thirdParty,
  }
})

describe("WalletRegistryGovernance", async () => {
  let governance: SignerWithAddress
  let walletRegistry: WalletRegistry
  let walletRegistryGovernance: WalletRegistryGovernance
  let thirdParty: SignerWithAddress

  const initialMinimumAuthorization = to1e18(40000)
  const initialAuthorizationDecreaseDelay = 3888000 // 45 days
  const initialAuthorizationDecreaseChangePeriod = 3888000 // 45 days
  const initialMaliciousDkgResultSlashingAmount = to1e18(400)
  const initialMaliciousDkgResultNotificationRewardMultiplier = 100
  const initialDkgResultSubmissionGas = 290_000
  const initialDkgResultApprovalGasOffset = 72_000
  const initialNotifyOperatorInactivityGasOffset = 93_000
  const initialNotifySeedTimeoutGasOffset = 7_250
  const initialNotifyDkgTimeoutNegativeGasOffset = 2_300
  const initialSortitionPoolRewardsBanDuration = 1209600 // 14 days

  before("load test fixture", async () => {
    // eslint-disable-next-line @typescript-eslint/no-extra-semi
    ;({ walletRegistry, walletRegistryGovernance, governance, thirdParty } =
      await fixture())
  })

  describe("upgradeRandomBeacon", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .upgradeRandomBeacon(thirdParty.address)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      context("when new address is zero", () => {
        it("should revert when a new random beacon address is zero", async () => {
          await expect(
            walletRegistryGovernance
              .connect(governance)
              .upgradeRandomBeacon(ethers.constants.AddressZero)
          ).to.be.revertedWith("New random beacon address cannot be zero")
        })
      })

      context("when new address is not zero", () => {
        before(async () => {
          await createSnapshot()

          tx = await walletRegistryGovernance
            .connect(governance)
            .upgradeRandomBeacon(thirdParty.address)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the random beacon", async () => {
          expect(await walletRegistry.randomBeacon()).to.be.equal(
            thirdParty.address
          )
        })

        it("should emit RandomBeaconUpgraded event", async () => {
          await expect(tx)
            .to.emit(walletRegistry, "RandomBeaconUpgraded")
            .withArgs(thirdParty.address)
        })
      })
    })
  })

  describe("initializeWalletOwner", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .initializeWalletOwner(thirdParty.address)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      context("when new address is zero", () => {
        it("should revert when a new address is zero", async () => {
          await expect(
            walletRegistryGovernance
              .connect(governance)
              .initializeWalletOwner(ethers.constants.AddressZero)
          ).to.be.revertedWith("Wallet Owner address cannot be zero")
        })
      })

      context("when new address is not zero", () => {
        before(async () => {
          await createSnapshot()

          tx = await walletRegistryGovernance
            .connect(governance)
            .initializeWalletOwner(thirdParty.address)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the wallet owner", async () => {
          expect(await walletRegistry.walletOwner()).to.be.equal(
            thirdParty.address
          )
        })

        it("should emit WalletOwnerUpdated event", async () => {
          await expect(tx)
            .to.emit(walletRegistry, "WalletOwnerUpdated")
            .withArgs(thirdParty.address)
        })

        it("should revert when called for the second time", async () => {
          await expect(
            walletRegistryGovernance
              .connect(governance)
              .initializeWalletOwner(thirdParty.address)
          ).to.be.revertedWith("Wallet Owner already initialized")
        })
      })
    })
  })

  describe("beginGovernanceDelayUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginGovernanceDelayUpdate(1)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginGovernanceDelayUpdate(1337)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the governance delay", async () => {
        expect(await walletRegistryGovernance.governanceDelay()).to.be.equal(
          constants.governanceDelay
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingGovernanceDelayUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit GovernanceDelayUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(walletRegistryGovernance, "GovernanceDelayUpdateStarted")
          .withArgs(1337, blockTimestamp)
      })
    })
  })

  describe("finalizeGovernanceDelayUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeGovernanceDelayUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeGovernanceDelayUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      before(async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginGovernanceDelayUpdate(7331)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeGovernanceDelayUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginGovernanceDelayUpdate(7331)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeGovernanceDelayUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the governance delay", async () => {
          expect(await walletRegistryGovernance.governanceDelay()).to.be.equal(
            7331
          )
        })

        it("should emit GovernanceDelayUpdated event", async () => {
          await expect(tx)
            .to.emit(walletRegistryGovernance, "GovernanceDelayUpdated")
            .withArgs(7331)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingGovernanceDelayUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginWalletRegistryGovernanceTransfer", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginWalletRegistryGovernanceTransfer(
              "0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537"
            )
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginWalletRegistryGovernanceTransfer(
            "0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537"
          )
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("when new governance is the zero address", () => {
        it("should revert", async () => {
          await expect(
            walletRegistryGovernance
              .connect(governance)
              .beginWalletRegistryGovernanceTransfer(
                ethers.constants.AddressZero
              )
          ).to.be.revertedWith(
            "New wallet registry governance address cannot be zero"
          )
        })
      })

      it("should not transfer the governance", async () => {
        expect(await walletRegistry.governance()).to.be.equal(
          walletRegistryGovernance.address
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingWalletRegistryGovernanceTransferDelayTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit WalletRegistryGovernanceTransferStarted", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "WalletRegistryGovernanceTransferStarted"
          )
          .withArgs(
            "0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537",
            blockTimestamp
          )
      })
    })
  })

  describe("finalizeWalletRegistryGovernanceTransfer", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeWalletRegistryGovernanceTransfer()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeWalletRegistryGovernanceTransfer()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      before(async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginWalletRegistryGovernanceTransfer(
            "0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537"
          )

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeWalletRegistryGovernanceTransfer()
        ).to.be.revertedWith("Governance delay has not elapsed")
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginWalletRegistryGovernanceTransfer(
              "0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537"
            )

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeWalletRegistryGovernanceTransfer()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should transfer wallet registry governance", async () => {
          expect(await walletRegistry.governance()).to.be.equal(
            "0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537"
          )
        })

        it("should emit WalletRegistryGovernanceTransferred event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "WalletRegistryGovernanceTransferred"
            )
            .withArgs("0x00Ea7D21bcCEeD400aCe08B583554aA619D3e537")
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingWalletRegistryGovernanceTransferDelayTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginWalletOwnerUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginWalletOwnerUpdate(thirdParty.address)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginWalletOwnerUpdate(thirdParty.address)
      })

      after(async () => {
        await restoreSnapshot()
      })

      describe("when new wallet owner is zero address", async () => {
        it("should revert", async () => {
          await expect(
            walletRegistryGovernance
              .connect(governance)
              .beginWalletOwnerUpdate(ethers.constants.AddressZero)
          ).to.be.revertedWith("New wallet owner address cannot be zero")
        })
      })

      it("should not update the wallet owner", async () => {
        expect(await walletRegistry.walletOwner()).to.be.equal(
          ethers.constants.AddressZero
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingWalletOwnerUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the WalletOwnerUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(walletRegistryGovernance, "WalletOwnerUpdateStarted")
          .withArgs(thirdParty.address, blockTimestamp)
      })
    })
  })

  describe("finalizeWalletOwnerUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeWalletOwnerUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeWalletOwnerUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginWalletOwnerUpdate(thirdParty.address)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeWalletOwnerUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginWalletOwnerUpdate(thirdParty.address)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeWalletOwnerUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the wallet owner", async () => {
          expect(await walletRegistry.walletOwner()).to.be.equal(
            thirdParty.address
          )
        })

        it("should emit WalletOwnerUpdated event", async () => {
          await expect(tx)
            .to.emit(walletRegistryGovernance, "WalletOwnerUpdated")
            .withArgs(thirdParty.address)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingWalletOwnerUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginMinimumAuthorizationUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginMinimumAuthorizationUpdate(123)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginMinimumAuthorizationUpdate(123)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the minimum authorization amount", async () => {
        expect(await walletRegistry.minimumAuthorization()).to.be.equal(
          initialMinimumAuthorization
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingMimimumAuthorizationUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the MinimumAuthorizationUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "MinimumAuthorizationUpdateStarted"
          )
          .withArgs(123, blockTimestamp)
      })
    })
  })

  describe("finalizeMinimumAuthorizationUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeMinimumAuthorizationUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeMinimumAuthorizationUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginMinimumAuthorizationUpdate(123)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeMinimumAuthorizationUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginMinimumAuthorizationUpdate(123)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeMinimumAuthorizationUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the minimum authorization amount", async () => {
          expect(await walletRegistry.minimumAuthorization()).to.be.equal(123)
        })

        it("should emit MinimumAuthorizationUpdated event", async () => {
          await expect(tx)
            .to.emit(walletRegistryGovernance, "MinimumAuthorizationUpdated")
            .withArgs(123)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingMimimumAuthorizationUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginAuthorizationDecreaseDelayUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginAuthorizationDecreaseDelayUpdate(123)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginAuthorizationDecreaseDelayUpdate(123)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the authorization decrease delay", async () => {
        const { authorizationDecreaseDelay } =
          await walletRegistry.authorizationParameters()
        expect(authorizationDecreaseDelay).to.be.equal(
          initialAuthorizationDecreaseDelay
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingAuthorizationDecreaseDelayUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the AuthorizationDecreaseDelayUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "AuthorizationDecreaseDelayUpdateStarted"
          )
          .withArgs(123, blockTimestamp)
      })
    })
  })

  describe("finalizeAuthorizationDecreaseDelayUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeAuthorizationDecreaseDelayUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeAuthorizationDecreaseDelayUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginAuthorizationDecreaseDelayUpdate(123)
        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeAuthorizationDecreaseDelayUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginAuthorizationDecreaseDelayUpdate(123)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeAuthorizationDecreaseDelayUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the authorization decrease delay", async () => {
          const { authorizationDecreaseDelay } =
            await walletRegistry.authorizationParameters()
          expect(authorizationDecreaseDelay).to.be.equal(123)
        })

        it("should emit AuthorizationDecreaseDelayUpdated event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "AuthorizationDecreaseDelayUpdated"
            )
            .withArgs(123)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingAuthorizationDecreaseDelayUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginAuthorizationDecreaseChangePeriodUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginAuthorizationDecreaseChangePeriodUpdate(123)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginAuthorizationDecreaseChangePeriodUpdate(123)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the authorization decrease change period", async () => {
        const { authorizationDecreaseChangePeriod } =
          await walletRegistry.authorizationParameters()
        expect(authorizationDecreaseChangePeriod).to.be.equal(
          initialAuthorizationDecreaseChangePeriod
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingAuthorizationDecreaseChangePeriodUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the AuthorizationDecreaseChangePeriodUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "AuthorizationDecreaseChangePeriodUpdateStarted"
          )
          .withArgs(123, blockTimestamp)
      })
    })
  })

  describe("finalizeAuthorizationDecreaseChangePeriodUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeAuthorizationDecreaseChangePeriodUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeAuthorizationDecreaseChangePeriodUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginAuthorizationDecreaseChangePeriodUpdate(123)
        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeAuthorizationDecreaseChangePeriodUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginAuthorizationDecreaseChangePeriodUpdate(123)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeAuthorizationDecreaseChangePeriodUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the authorization decrease change period", async () => {
          const { authorizationDecreaseChangePeriod } =
            await walletRegistry.authorizationParameters()
          expect(authorizationDecreaseChangePeriod).to.be.equal(123)
        })

        it("should emit AuthorizationDecreaseChangePeriodUpdated event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "AuthorizationDecreaseChangePeriodUpdated"
            )
            .withArgs(123)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingAuthorizationDecreaseChangePeriodUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginMaliciousDkgResultSlashingAmountUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginMaliciousDkgResultSlashingAmountUpdate(123)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginMaliciousDkgResultSlashingAmountUpdate(123)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the malicious DKG result slashing amount", async () => {
        const maliciousDkgResultSlashingAmount =
          await walletRegistry.slashingParameters()
        expect(maliciousDkgResultSlashingAmount).to.be.equal(
          initialMaliciousDkgResultSlashingAmount
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingMaliciousDkgResultSlashingAmountUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the MaliciousDkgResultSlashingAmountUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "MaliciousDkgResultSlashingAmountUpdateStarted"
          )
          .withArgs(123, blockTimestamp)
      })
    })
  })

  describe("finalizeMaliciousDkgResultSlashingAmountUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeMaliciousDkgResultSlashingAmountUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeMaliciousDkgResultSlashingAmountUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginMaliciousDkgResultSlashingAmountUpdate(123)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeMaliciousDkgResultSlashingAmountUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginMaliciousDkgResultSlashingAmountUpdate(123)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeMaliciousDkgResultSlashingAmountUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the malicious DKG result slashing amount", async () => {
          const maliciousDkgResultSlashingAmount =
            await walletRegistry.slashingParameters()
          expect(maliciousDkgResultSlashingAmount).to.be.equal(123)
        })

        it("should emit MaliciousDkgResultSlashingAmountUpdated event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "MaliciousDkgResultSlashingAmountUpdated"
            )
            .withArgs(123)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingMaliciousDkgResultSlashingAmountUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginDkgResultSubmissionGasUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginDkgResultSubmissionGasUpdate(100)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginDkgResultSubmissionGasUpdate(100)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG submission result gas", async () => {
        const { dkgResultSubmissionGas } = await walletRegistry.gasParameters()
        expect(dkgResultSubmissionGas).to.be.equal(
          initialDkgResultSubmissionGas
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingDkgResultSubmissionGasUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the DkgResultSubmissionGasUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "DkgResultSubmissionGasUpdateStarted"
          )
          .withArgs(100, blockTimestamp)
      })
    })
  })

  describe("finalizeDkgResultSubmissionGasUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeDkgResultSubmissionGasUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultSubmissionGasUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginDkgResultSubmissionGasUpdate(100)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultSubmissionGasUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginDkgResultSubmissionGasUpdate(100)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultSubmissionGasUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG result submission gas", async () => {
          const { dkgResultSubmissionGas } =
            await walletRegistry.gasParameters()
          expect(dkgResultSubmissionGas).to.be.equal(100)
        })

        it("should emit DkgResultSubmissionGasUpdated event", async () => {
          await expect(tx)
            .to.emit(walletRegistryGovernance, "DkgResultSubmissionGasUpdated")
            .withArgs(100)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingDkgResultSubmissionGasUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginDkgResultApprovalGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginDkgResultApprovalGasOffsetUpdate(100)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginDkgResultApprovalGasOffsetUpdate(100)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG approval gas offset", async () => {
        const { dkgResultApprovalGasOffset } =
          await walletRegistry.gasParameters()
        expect(dkgResultApprovalGasOffset).to.be.equal(
          initialDkgResultApprovalGasOffset
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingDkgResultApprovalGasOffsetUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the DkgResultApprovalGasOffsetUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "DkgResultApprovalGasOffsetUpdateStarted"
          )
          .withArgs(100, blockTimestamp)
      })
    })
  })

  describe("finalizeDkgResultApprovalGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeDkgResultApprovalGasOffsetUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultApprovalGasOffsetUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginDkgResultApprovalGasOffsetUpdate(100)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultApprovalGasOffsetUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginDkgResultApprovalGasOffsetUpdate(100)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultApprovalGasOffsetUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG result approval gas offset", async () => {
          const { dkgResultApprovalGasOffset } =
            await walletRegistry.gasParameters()
          expect(dkgResultApprovalGasOffset).to.be.equal(100)
        })

        it("should emit DkgResultApprovalGasOffsetUpdated event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "DkgResultApprovalGasOffsetUpdated"
            )
            .withArgs(100)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingDkgResultApprovalGasOffsetUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginMaliciousDkgResultNotificationRewardMultiplierUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginMaliciousDkgResultNotificationRewardMultiplierUpdate(100)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when called with value >100", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .beginMaliciousDkgResultNotificationRewardMultiplierUpdate(101)
        ).to.be.revertedWith("Maximum value is 100")
      })
    })

    context("when the caller is the owner and value is correct", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginMaliciousDkgResultNotificationRewardMultiplierUpdate(100)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG malicious result notification reward multiplier", async () => {
        const { maliciousDkgResultNotificationRewardMultiplier } =
          await walletRegistry.rewardParameters()
        expect(maliciousDkgResultNotificationRewardMultiplier).to.be.equal(
          initialMaliciousDkgResultNotificationRewardMultiplier
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingMaliciousDkgResultNotificationRewardMultiplierUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the MaliciousDkgResultNotificationRewardMultiplierUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "MaliciousDkgResultNotificationRewardMultiplierUpdateStarted"
          )
          .withArgs(100, blockTimestamp)
      })
    })
  })

  describe("finalizeMaliciousDkgResultNotificationRewardMultiplierUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeMaliciousDkgResultNotificationRewardMultiplierUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeMaliciousDkgResultNotificationRewardMultiplierUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginMaliciousDkgResultNotificationRewardMultiplierUpdate(100)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeMaliciousDkgResultNotificationRewardMultiplierUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginMaliciousDkgResultNotificationRewardMultiplierUpdate(100)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeMaliciousDkgResultNotificationRewardMultiplierUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG malicious result notification reward multiplier", async () => {
          const { maliciousDkgResultNotificationRewardMultiplier } =
            await walletRegistry.rewardParameters()
          expect(maliciousDkgResultNotificationRewardMultiplier).to.be.equal(
            100
          )
        })

        it("should emit MaliciousDkgResultNotificationRewardMultiplierUpdated event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "MaliciousDkgResultNotificationRewardMultiplierUpdated"
            )
            .withArgs(100)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingMaliciousDkgResultNotificationRewardMultiplierUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginSortitionPoolRewardsBanDurationUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginSortitionPoolRewardsBanDurationUpdate(86400)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginSortitionPoolRewardsBanDurationUpdate(86400)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the sortition pool rewards ban duration", async () => {
        const { sortitionPoolRewardsBanDuration } =
          await walletRegistry.rewardParameters()
        expect(sortitionPoolRewardsBanDuration).to.be.equal(
          initialSortitionPoolRewardsBanDuration
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingSortitionPoolRewardsBanDurationUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the SortitionPoolRewardsBanDurationUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "SortitionPoolRewardsBanDurationUpdateStarted"
          )
          .withArgs(86400, blockTimestamp)
      })
    })
  })

  describe("finalizeSortitionPoolRewardsBanDurationUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeSortitionPoolRewardsBanDurationUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeSortitionPoolRewardsBanDurationUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginSortitionPoolRewardsBanDurationUpdate(86400)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeSortitionPoolRewardsBanDurationUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginSortitionPoolRewardsBanDurationUpdate(86400)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeSortitionPoolRewardsBanDurationUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the sortition pool rewards ban duration", async () => {
          const { sortitionPoolRewardsBanDuration } =
            await walletRegistry.rewardParameters()
          expect(sortitionPoolRewardsBanDuration).to.be.equal(86400)
        })

        it("should emit SortitionPoolRewardsBanDurationUpdated event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "SortitionPoolRewardsBanDurationUpdated"
            )
            .withArgs(86400)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingSortitionPoolRewardsBanDurationUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginDkgSeedTimeoutUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginDkgSeedTimeoutUpdate(11)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update value is equal 0", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .beginDkgSeedTimeoutUpdate(0)
        ).to.be.revertedWith("DKG seed timeout must be > 0")
      })
    })

    context("when the update value is at least 1", () => {
      before(async () => {
        await createSnapshot()
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should accept the value", async () => {
        await createSnapshot()

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .beginDkgSeedTimeoutUpdate(1)
        ).not.to.be.reverted
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .beginDkgSeedTimeoutUpdate(11)
        ).not.to.be.reverted

        await restoreSnapshot()
      })
    })

    context("when the caller is the owner and the value is correct", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginDkgSeedTimeoutUpdate(11)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG seed timeout", async () => {
        expect((await walletRegistry.dkgParameters()).seedTimeout).to.be.equal(
          params.dkgSeedTimeout
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingDkgSeedTimeoutUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the DkgSeedTimeoutUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(walletRegistryGovernance, "DkgSeedTimeoutUpdateStarted")
          .withArgs(11, blockTimestamp)
      })
    })
  })

  describe("finalizeDkgSeedTimeoutUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeDkgSeedTimeoutUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgSeedTimeoutUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginDkgSeedTimeoutUpdate(11)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgSeedTimeoutUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginDkgSeedTimeoutUpdate(11)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeDkgSeedTimeoutUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG seed timeout", async () => {
          expect(
            (await walletRegistry.dkgParameters()).seedTimeout
          ).to.be.equal(11)
        })

        it("should emit DkgSeedTimeoutUpdated event", async () => {
          await expect(tx)
            .to.emit(walletRegistryGovernance, "DkgSeedTimeoutUpdated")
            .withArgs(11)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingDkgSeedTimeoutUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginDkgResultChallengePeriodLengthUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginDkgResultChallengePeriodLengthUpdate(11)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update value is less than 10", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .beginDkgResultChallengePeriodLengthUpdate(9)
        ).to.be.revertedWith("DKG result challenge period length must be >= 10")
      })
    })

    context("when the update value is at least 10", () => {
      it("should accept the value", async () => {
        await createSnapshot()

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .beginDkgResultChallengePeriodLengthUpdate(10)
        ).to.not.be.reverted

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .beginDkgResultChallengePeriodLengthUpdate(11)
        ).to.not.be.reverted

        await restoreSnapshot()
      })
    })

    context("when the caller is the owner and the value is correct", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginDkgResultChallengePeriodLengthUpdate(11)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG result challenge period length", async () => {
        expect(
          (await walletRegistry.dkgParameters()).resultChallengePeriodLength
        ).to.be.equal(params.dkgResultChallengePeriodLength)
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingDkgResultChallengePeriodLengthUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the DkgResultChallengePeriodLengthUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "DkgResultChallengePeriodLengthUpdateStarted"
          )
          .withArgs(11, blockTimestamp)
      })
    })
  })

  describe("finalizeDkgResultChallengePeriodLengthUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeDkgResultChallengePeriodLengthUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultChallengePeriodLengthUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginDkgResultChallengePeriodLengthUpdate(11)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultChallengePeriodLengthUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginDkgResultChallengePeriodLengthUpdate(11)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultChallengePeriodLengthUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG result challenge period length", async () => {
          expect(
            (await walletRegistry.dkgParameters()).resultChallengePeriodLength
          ).to.be.equal(11)
        })

        it("should emit DkgResultChallengePeriodLengthUpdated event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "DkgResultChallengePeriodLengthUpdated"
            )
            .withArgs(11)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingDkgResultChallengePeriodLengthUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginDkgResultChallengeExtraGasUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginDkgResultChallengeExtraGasUpdate(1)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginDkgResultChallengeExtraGasUpdate(1337)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG result challenge extra gas", async () => {
        expect(
          (await walletRegistry.dkgParameters()).resultChallengeExtraGas
        ).to.be.equal(params.dkgResultChallengeExtraGas)
      })

      it("should emit DkgResultChallengeExtraGasUpdateStarted", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "DkgResultChallengeExtraGasUpdateStarted"
          )
          .withArgs(1337, blockTimestamp)
      })
    })
  })

  describe("finalizeDkgResultChallengeExtraGasUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeDkgResultChallengeExtraGasUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultChallengeExtraGasUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginDkgResultChallengeExtraGasUpdate(1)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultChallengeExtraGasUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginDkgResultChallengeExtraGasUpdate(1337)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultChallengeExtraGasUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG result challenge extra gas", async () => {
          expect(
            (await walletRegistry.dkgParameters()).resultChallengeExtraGas
          ).to.be.equal(1337)
        })

        it("should emit DkgResultChallengeExtraGasUpdated event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "DkgResultChallengeExtraGasUpdated"
            )
            .withArgs(1337)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingDkgResultChallengeExtraGasUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginDkgResultSubmissionTimeoutUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginDkgResultSubmissionTimeoutUpdate(1)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update value is zero", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .beginDkgResultSubmissionTimeoutUpdate(0)
        ).to.be.revertedWith("DKG result submission timeout must be > 0")
      })
    })

    context("when the update value is at least one", () => {
      it("should accept the value", async () => {
        await createSnapshot()

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .beginDkgResultSubmissionTimeoutUpdate(1)
        ).to.not.be.reverted
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .beginDkgResultSubmissionTimeoutUpdate(2)
        ).to.not.be.reverted

        await restoreSnapshot()
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginDkgResultSubmissionTimeoutUpdate(1)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG result submission timeout", async () => {
        expect(
          (await walletRegistry.dkgParameters()).resultSubmissionTimeout
        ).to.be.equal(params.dkgResultSubmissionTimeout)
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingDkgResultSubmissionTimeoutUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the DkgResultSubmissionTimeoutUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "DkgResultSubmissionTimeoutUpdateStarted"
          )
          .withArgs(1, blockTimestamp)
      })
    })
  })

  describe("finalizeDkgResultSubmissionTimeoutUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeDkgResultSubmissionTimeoutUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultSubmissionTimeoutUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginDkgResultSubmissionTimeoutUpdate(1)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultSubmissionTimeoutUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginDkgResultSubmissionTimeoutUpdate(10)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeDkgResultSubmissionTimeoutUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG result submission timeout", async () => {
          expect(
            (await walletRegistry.dkgParameters()).resultSubmissionTimeout
          ).to.be.equal(10)
        })

        it("should emit DkgResultSubmissionTimeoutUpdated event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "DkgResultSubmissionTimeoutUpdated"
            )
            .withArgs(10)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingDkgResultSubmissionTimeoutUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginDkgSubmitterPrecedencePeriodLengthUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginDkgSubmitterPrecedencePeriodLengthUpdate(1)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update value is zero", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .beginDkgSubmitterPrecedencePeriodLengthUpdate(0)
        ).to.be.revertedWith(
          "DKG submitter precedence period length must be > 0"
        )
      })
    })

    context("when the update value is at least one", () => {
      it("should accept the value", async () => {
        await createSnapshot()

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .beginDkgSubmitterPrecedencePeriodLengthUpdate(1)
        ).to.not.be.reverted
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .beginDkgSubmitterPrecedencePeriodLengthUpdate(2)
        ).to.not.be.reverted

        await restoreSnapshot()
      })
    })

    context("when the caller is the owner and the value is correct", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginDkgSubmitterPrecedencePeriodLengthUpdate(1)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the DKG submitter precedence period length", async () => {
        expect(
          (await walletRegistry.dkgParameters()).submitterPrecedencePeriodLength
        ).to.be.equal(params.dkgSubmitterPrecedencePeriodLength)
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingDkgSubmitterPrecedencePeriodLengthUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the DkgSubmitterPrecedencePeriodLengthUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "DkgSubmitterPrecedencePeriodLengthUpdateStarted"
          )
          .withArgs(1, blockTimestamp)
      })
    })
  })

  describe("finalizeDkgSubmitterPrecedencePeriodLengthUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeDkgSubmitterPrecedencePeriodLengthUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgSubmitterPrecedencePeriodLengthUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginDkgSubmitterPrecedencePeriodLengthUpdate(1)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeDkgSubmitterPrecedencePeriodLengthUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginDkgSubmitterPrecedencePeriodLengthUpdate(10)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeDkgSubmitterPrecedencePeriodLengthUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the DKG submitter precedence period length", async () => {
          expect(
            (await walletRegistry.dkgParameters())
              .submitterPrecedencePeriodLength
          ).to.be.equal(10)
        })

        it("should emit DkgSubmitterPrecedencePeriodLengthUpdated event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "DkgSubmitterPrecedencePeriodLengthUpdated"
            )
            .withArgs(10)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingDkgSubmitterPrecedencePeriodLengthUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginReimbursementPoolUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginReimbursementPoolUpdate(thirdParty.address)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction
      let reimbursementPoolAddress: string

      before(async () => {
        await createSnapshot()

        reimbursementPoolAddress = await walletRegistry.reimbursementPool()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginReimbursementPoolUpdate(thirdParty.address)
      })

      after(async () => {
        await restoreSnapshot()
      })

      describe("when the new reimbursement pool is zero address", async () => {
        it("should revert", async () => {
          await expect(
            walletRegistryGovernance
              .connect(governance)
              .beginReimbursementPoolUpdate(ethers.constants.AddressZero)
          ).to.be.revertedWith("New reimbursement pool address cannot be zero")
        })
      })

      it("should not update the reimbursement pool", async () => {
        expect(await walletRegistry.reimbursementPool()).to.be.equal(
          reimbursementPoolAddress
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingReimbursementPoolUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the ReimbursementPoolUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(walletRegistryGovernance, "ReimbursementPoolUpdateStarted")
          .withArgs(thirdParty.address, blockTimestamp)
      })
    })
  })

  describe("finalizeReimbursementPoolUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeReimbursementPoolUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeReimbursementPoolUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginReimbursementPoolUpdate(thirdParty.address)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeReimbursementPoolUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginReimbursementPoolUpdate(thirdParty.address)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeReimbursementPoolUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the reimbursement pool", async () => {
          expect(await walletRegistry.reimbursementPool()).to.be.equal(
            thirdParty.address
          )
        })

        it("should emit ReimbursementPoolUpdated event", async () => {
          await expect(tx)
            .to.emit(walletRegistryGovernance, "ReimbursementPoolUpdated")
            .withArgs(thirdParty.address)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingReimbursementPoolUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginNotifyOperatorInactivityGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginNotifyOperatorInactivityGasOffsetUpdate(100)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginNotifyOperatorInactivityGasOffsetUpdate(100)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the operator inactivity gas offset", async () => {
        const { notifyOperatorInactivityGasOffset } =
          await walletRegistry.gasParameters()
        expect(notifyOperatorInactivityGasOffset).to.be.equal(
          initialNotifyOperatorInactivityGasOffset
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingNotifyOperatorInactivityGasOffsetUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the NotifyOperatorInactivityGasOffsetUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "NotifyOperatorInactivityGasOffsetUpdateStarted"
          )
          .withArgs(100, blockTimestamp)
      })
    })
  })

  describe("finalizeNotifyOperatorInactivityGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeNotifyOperatorInactivityGasOffsetUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeNotifyOperatorInactivityGasOffsetUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginNotifyOperatorInactivityGasOffsetUpdate(100)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeNotifyOperatorInactivityGasOffsetUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginNotifyOperatorInactivityGasOffsetUpdate(100)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeNotifyOperatorInactivityGasOffsetUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the operator inactivity gas offset", async () => {
          const { notifyOperatorInactivityGasOffset } =
            await walletRegistry.gasParameters()
          expect(notifyOperatorInactivityGasOffset).to.be.equal(100)
        })

        it("should emit NotifyOperatorInactivityGasOffsetUpdated event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "NotifyOperatorInactivityGasOffsetUpdated"
            )
            .withArgs(100)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingNotifyOperatorInactivityGasOffsetUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginNotifySeedTimeoutGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginNotifySeedTimeoutGasOffsetUpdate(100)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginNotifySeedTimeoutGasOffsetUpdate(100)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the notify seed timeout gas offset", async () => {
        const { notifySeedTimeoutGasOffset } =
          await walletRegistry.gasParameters()
        expect(notifySeedTimeoutGasOffset).to.be.equal(
          initialNotifySeedTimeoutGasOffset
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingNotifySeedTimeoutGasOffsetUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the NotifySeedTimeoutGasOffsetUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "NotifySeedTimeoutGasOffsetUpdateStarted"
          )
          .withArgs(100, blockTimestamp)
      })
    })
  })

  describe("finalizeNotifySeedTimeoutGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeNotifySeedTimeoutGasOffsetUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeNotifySeedTimeoutGasOffsetUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginNotifySeedTimeoutGasOffsetUpdate(100)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeNotifySeedTimeoutGasOffsetUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginNotifySeedTimeoutGasOffsetUpdate(100)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeNotifySeedTimeoutGasOffsetUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the notify seed timeout gas offset", async () => {
          const { notifySeedTimeoutGasOffset } =
            await walletRegistry.gasParameters()
          expect(notifySeedTimeoutGasOffset).to.be.equal(100)
        })

        it("should emit NotifySeedTimeoutGasOffsetUpdated event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "NotifySeedTimeoutGasOffsetUpdated"
            )
            .withArgs(100)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingNotifySeedTimeoutGasOffsetUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("beginNotifyDkgTimeoutNegativeGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .beginNotifyDkgTimeoutNegativeGasOffsetUpdate(100)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the caller is the owner", () => {
      let tx: ContractTransaction

      before(async () => {
        await createSnapshot()

        tx = await walletRegistryGovernance
          .connect(governance)
          .beginNotifyDkgTimeoutNegativeGasOffsetUpdate(100)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should not update the notify DKG timeout negative gas offset", async () => {
        const { notifyDkgTimeoutNegativeGasOffset } =
          await walletRegistry.gasParameters()
        expect(notifyDkgTimeoutNegativeGasOffset).to.be.equal(
          initialNotifyDkgTimeoutNegativeGasOffset
        )
      })

      it("should start the governance delay timer", async () => {
        expect(
          await walletRegistryGovernance.getRemainingNotifyDkgTimeoutNegativeGasOffsetUpdateTime()
        ).to.be.equal(constants.governanceDelay)
      })

      it("should emit the NotifyDkgTimeoutNegativeGasOffsetUpdateStarted event", async () => {
        const blockTimestamp = (await ethers.provider.getBlock(tx.blockNumber))
          .timestamp
        await expect(tx)
          .to.emit(
            walletRegistryGovernance,
            "NotifyDkgTimeoutNegativeGasOffsetUpdateStarted"
          )
          .withArgs(100, blockTimestamp)
      })
    })
  })

  describe("finalizeNotifyDkgTimeoutNegativeGasOffsetUpdate", () => {
    context("when the caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .finalizeNotifyDkgTimeoutNegativeGasOffsetUpdate()
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    context("when the update process is not initialized", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeNotifyDkgTimeoutNegativeGasOffsetUpdate()
        ).to.be.revertedWith("Change not initiated")
      })
    })

    context("when the governance delay has not passed", () => {
      it("should revert", async () => {
        await createSnapshot()

        await walletRegistryGovernance
          .connect(governance)
          .beginNotifyDkgTimeoutNegativeGasOffsetUpdate(100)

        await helpers.time.increaseTime(constants.governanceDelay - 60) // -1min

        await expect(
          walletRegistryGovernance
            .connect(governance)
            .finalizeNotifyDkgTimeoutNegativeGasOffsetUpdate()
        ).to.be.revertedWith("Governance delay has not elapsed")

        await restoreSnapshot()
      })
    })

    context(
      "when the update process is initialized and governance delay passed",
      () => {
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistryGovernance
            .connect(governance)
            .beginNotifyDkgTimeoutNegativeGasOffsetUpdate(100)

          await helpers.time.increaseTime(constants.governanceDelay)

          tx = await walletRegistryGovernance
            .connect(governance)
            .finalizeNotifyDkgTimeoutNegativeGasOffsetUpdate()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should update the notify DKG timeout negative gas offset", async () => {
          const { notifyDkgTimeoutNegativeGasOffset } =
            await walletRegistry.gasParameters()
          expect(notifyDkgTimeoutNegativeGasOffset).to.be.equal(100)
        })

        it("should emit NotifyDkgTimeoutNegativeGasOffsetUpdated event", async () => {
          await expect(tx)
            .to.emit(
              walletRegistryGovernance,
              "NotifyDkgTimeoutNegativeGasOffsetUpdated"
            )
            .withArgs(100)
        })

        it("should reset the governance delay timer", async () => {
          await expect(
            walletRegistryGovernance.getRemainingNotifyDkgTimeoutNegativeGasOffsetUpdateTime()
          ).to.be.revertedWith("Change not initiated")
        })
      }
    )
  })

  describe("withdrawIneligibleRewards", () => {
    context("when caller is not the owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistryGovernance
            .connect(thirdParty)
            .withdrawIneligibleRewards(thirdParty.address)
        ).to.be.revertedWith("Ownable: caller is not the owner")
      })
    })

    // The actual functionality is tested in WalletRegistry.Rewards.test.ts
  })
})

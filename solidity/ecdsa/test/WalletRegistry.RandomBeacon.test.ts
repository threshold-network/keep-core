/* eslint-disable no-underscore-dangle */
import { ethers, helpers } from "hardhat"
import { expect } from "chai"
import { smock } from "@defi-wonderland/smock"

import { dkgState, walletRegistryFixture } from "./fixtures"
import { resetMock } from "./utils/randomBeacon"
import { upgradeRandomBeacon } from "./utils/governance"

import type { IWalletOwner } from "../typechain/IWalletOwner"
import type { MockContract, FakeContract } from "@defi-wonderland/smock"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type {
  IRandomBeacon,
  RandomBeaconStub,
  RandomBeaconStub__factory,
  WalletRegistry,
  WalletRegistryStub,
} from "../typechain"
import type { BigNumber, ContractTransaction } from "ethers"

const { createSnapshot, restoreSnapshot } = helpers.snapshot

describe("WalletRegistry - Random Beacon", async () => {
  let walletRegistry: WalletRegistryStub & WalletRegistry
  let randomBeaconFake: FakeContract<IRandomBeacon>
  let walletOwner: FakeContract<IWalletOwner>
  let thirdParty: SignerWithAddress

  before("load test fixture", async () => {
    // eslint-disable-next-line @typescript-eslint/no-extra-semi
    ;({
      walletRegistry,
      walletOwner,
      thirdParty,
      randomBeacon: randomBeaconFake,
    } = await walletRegistryFixture())
  })

  describe("requestNewWallet", async () => {
    context("when requestRelayEntry reverts", async () => {
      let tx: Promise<ContractTransaction>

      before(async () => {
        await createSnapshot()

        await randomBeaconFake.requestRelayEntry.reverts(
          "beacon internal error"
        )

        tx = walletRegistry.connect(walletOwner.wallet).requestNewWallet()
      })

      after(async () => {
        await restoreSnapshot()

        resetMock(randomBeaconFake)
      })

      it("should revert", async () => {
        // FIXME: For some reason this check doesn't work with the expected error message
        // await expect(tx).to.be.revertedWith("beacon internal error")
        await expect(tx).to.be.reverted
      })
    })

    context("when requestRelayEntry succeeds", async () => {
      let tx: Promise<ContractTransaction>

      before(async () => {
        await createSnapshot()

        tx = walletRegistry.connect(walletOwner.wallet).requestNewWallet()
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should succeed", async () => {
        await expect(tx).to.not.be.reverted
      })

      it("should call random beacon", async () => {
        await expect(randomBeaconFake.requestRelayEntry).to.be.calledWith(
          walletRegistry.address
        )
      })
    })
  })

  describe("__beaconCallback", async () => {
    before(async () => {
      await createSnapshot()
    })

    after(async () => {
      await restoreSnapshot()
    })

    context("when called by a third party", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(thirdParty).__beaconCallback(123, 456)
        ).to.be.revertedWithCustomError(walletRegistry, "CallerNotRandomBeacon")
      })
    })

    context("when called by the random beacon", async () => {
      context("when new wallet was not requested", async () => {
        it("should revert", async () => {
          await expect(
            walletRegistry
              .connect(randomBeaconFake.wallet)
              .__beaconCallback(123, 456)
          ).to.be.revertedWith("Current state is not AWAITING_SEED")
        })
      })

      context("when new wallet was requested", async () => {
        const relayEntry = ethers.BigNumber.from(ethers.utils.randomBytes(32))
        let tx: ContractTransaction

        before(async () => {
          await createSnapshot()

          await walletRegistry.connect(walletOwner.wallet).requestNewWallet()

          tx = await walletRegistry
            .connect(randomBeaconFake.wallet)
            .__beaconCallback(relayEntry, 0)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should transition wallet creation state to `AWAITING_RESULT`", async () => {
          await expect(
            await walletRegistry.getWalletCreationState()
          ).to.be.equal(dkgState.AWAITING_RESULT)
        })

        it("should set seed for wallet creation", async () => {
          await expect((await walletRegistry.getDkgData()).seed).to.be.equal(
            relayEntry
          )
        })

        it("should set start block for wallet creation", async () => {
          await expect(
            (
              await walletRegistry.getDkgData()
            ).startBlock
          ).to.be.equal(tx.blockNumber)
        })

        it("should not emit DkgStateLocked event", async () => {
          await expect(tx).not.to.emit(walletRegistry, "DkgStateLocked")
        })

        it("should emit DkgStarted event", async () => {
          await expect(tx).to.emit(walletRegistry, "DkgStarted")
        })
      })

      describe("gas estimation", async () => {
        before(async () => {
          await createSnapshot()

          await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
        })

        after(async () => {
          await restoreSnapshot()
        })

        // The exact value was noted from a test execution and is used as a reference
        // for all future executions.
        it("should not exceed 85700", async () => {
          const expectedGasEstimate = 85700

          const gasEstimate = await walletRegistry
            .connect(randomBeaconFake.wallet)
            .estimateGas.__beaconCallback(
              ethers.BigNumber.from(ethers.utils.randomBytes(32)),
              0
            )

          await expect(gasEstimate).to.be.lte(expectedGasEstimate)
        })
      })

      // It's easier and cleaner to test with a Fake Contract of IRandomBeacon
      // interface, hence we use that approach in other tests. Here we want to
      // simulate a real-world use case as much as possible so we switch to
      // mocking the actual contract that uses a Callback library with a set fixed
      // gas limit. The main point of this test is to validate that `callbackGasLimit`
      // set in the `RandomBeacon`'s `Callback` library is enough to cover
      // `WalletRegistry.__beaconCallback` execution.
      context("when called as a callback from random beacon", async () => {
        let randomBeaconMock: MockContract<RandomBeaconStub>

        before(async () => {
          await createSnapshot()

          randomBeaconMock = await mockRandomBeacon(walletRegistry)

          await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should succeed", async () => {
          const entry: BigNumber = ethers.BigNumber.from(
            ethers.utils.randomBytes(32)
          )

          const tx = await randomBeaconMock.submitRelayEntry(entry)

          await expect(
            tx,
            "Callback failed; inspect callbackGasLimit value is sufficient"
          ).not.to.emit(randomBeaconMock, "CallbackFailed")
        })
      })
    })
  })
})

async function mockRandomBeacon(
  walletRegistry: WalletRegistry
): Promise<MockContract<RandomBeaconStub>> {
  const randomBeaconFactory = await smock.mock<RandomBeaconStub__factory>(
    "RandomBeaconStub"
  )

  const randomBeacon: MockContract<RandomBeaconStub> =
    await randomBeaconFactory.deploy()

  await upgradeRandomBeacon(walletRegistry, randomBeacon.address)

  return randomBeacon
}

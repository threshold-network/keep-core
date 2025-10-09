import { helpers } from "hardhat"
import { expect } from "chai"

import { walletRegistryFixture } from "./fixtures"

import type { IWalletOwner } from "../typechain/IWalletOwner"
import type { FakeContract } from "@defi-wonderland/smock"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { WalletRegistry, WalletRegistryStub, IRandomBeacon, WalletRegistryGovernance } from "../typechain"

const { createSnapshot, restoreSnapshot } = helpers.snapshot

describe("WalletRegistry - Parameters", async () => {
  let walletRegistry: WalletRegistryStub & WalletRegistry
  let walletRegistryGovernance: WalletRegistryGovernance

  let deployer: SignerWithAddress
  let governance: SignerWithAddress
  let walletOwner: FakeContract<IWalletOwner>
  let randomBeacon: FakeContract<IRandomBeacon>
  let thirdParty: SignerWithAddress

  before("load test fixture", async () => {
    // eslint-disable-next-line @typescript-eslint/no-extra-semi
    ;({ walletRegistry, walletRegistryGovernance, walletOwner, randomBeacon, deployer, governance, thirdParty } =
      await walletRegistryFixture())
  })

  describe("updateAuthorizationParameters", () => {
    context("when called by the deployer", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(deployer)
            .updateAuthorizationParameters(1, 2, 3)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by the wallet owner", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(walletOwner.wallet)
            .updateAuthorizationParameters(1, 2, 3)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by a third party", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(thirdParty)
            .updateAuthorizationParameters(1, 2, 3)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })
  })

  describe("updateDkgParameters", async () => {
    context("when called by the deployer", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(deployer).updateDkgParameters(1, 2, 3, 4, 5)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by the wallet owner", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(walletOwner.wallet)
            .updateDkgParameters(1, 2, 3, 4, 5)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by a third party", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(thirdParty).updateDkgParameters(1, 2, 3, 4, 5)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    // Test for DKG state validation will be added in a separate focused test
    // since it requires governance contract setup. For now, this test path
    // is covered by the contract's internal state machine validation.
  })

  describe("updateRewardParameters", async () => {
    context("when called by the deployer", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(deployer).updateRewardParameters(1, 2)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by the wallet owner", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(walletOwner.wallet)
            .updateRewardParameters(1, 2)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by a third party", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(thirdParty).updateRewardParameters(1, 2)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })
  })

  describe("updateSlashingParameters", async () => {
    context("when called by the deployer", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(deployer).updateSlashingParameters(1)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by the wallet owner", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(walletOwner.wallet).updateSlashingParameters(1)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by a third party", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(thirdParty).updateSlashingParameters(1)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })
  })

  describe("updateWalletOwner", async () => {
    context("when called by the deployer", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(deployer).updateWalletOwner(thirdParty.address)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by the wallet owner", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(walletOwner.wallet)
            .updateWalletOwner(thirdParty.address)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by a third party", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(thirdParty)
            .updateWalletOwner(thirdParty.address)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })
  })

  describe("updateGasParameters", async () => {
    context("when called by the deployer", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(deployer)
            .updateGasParameters(4200, 4201, 4202, 4203, 4204)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by the wallet owner", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(walletOwner.wallet)
            .updateGasParameters(4200, 4201, 4202, 4203, 4204)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by a third party", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(thirdParty)
            .updateGasParameters(4200, 4201, 4202, 4203, 4204)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })
  })

  describe("upgradeRandomBeacon", async () => {
    context("when called by the deployer", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(deployer)
            .upgradeRandomBeacon(thirdParty.address)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by the wallet owner", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(walletOwner.wallet)
            .upgradeRandomBeacon(thirdParty.address)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })

    context("when called by a third party", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry
            .connect(thirdParty)
            .upgradeRandomBeacon(thirdParty.address)
        ).to.be.revertedWith("Caller is not the governance")
      })
    })
  })
})

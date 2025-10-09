import { deployments, ethers, upgrades, helpers } from "hardhat"
import chai, { expect } from "chai"
import chaiAsPromised from "chai-as-promised"
import { keccak256 } from "ethers/lib/utils"

import { params, walletRegistryFixture } from "./fixtures"
import { noMisbehaved, signAndSubmitCorrectDkgResult } from "./utils/dkg"
import ecdsaData from "./data/ecdsa"
import { createNewWallet } from "./utils/wallets"

import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { WalletRegistry, WalletRegistryV2 } from "../typechain"
import type { FactoryOptions } from "hardhat/types"
import type { Contract } from "ethers"
import type { UpgradeProxyOptions } from "@openzeppelin/hardhat-upgrades/src/utils/options"

const { mineBlocksTo } = helpers.time
const { AddressZero } = ethers.constants

chai.use(chaiAsPromised)

describe("WalletRegistry - Upgrade", async () => {
  let proxyAdminOwner: SignerWithAddress
  let EcdsaInactivity: Contract

  before(async () => {
    // eslint-disable-next-line @typescript-eslint/no-extra-semi
    ;({ esdm: proxyAdminOwner } = await helpers.signers.getNamedSigners())
    EcdsaInactivity = await helpers.contracts.getContract("EcdsaInactivity")
  })

  describe("initializeV2", () => {
    describe("when allowlist address is zero", () => {
      it("should revert with AllowlistAddressZero", async () => {
        await deployments.fixture()

        await expect(
          upgradeProxy("WalletRegistry", "WalletRegistry", {
            factoryOpts: {
              libraries: { EcdsaInactivity: EcdsaInactivity.address },
              signer: proxyAdminOwner,
            },
            proxyOpts: {
              constructorArgs: [AddressZero, AddressZero],
              call: {
                fn: "initializeV2",
                args: [AddressZero], // Zero address should be rejected
              },
              unsafeAllow: ["external-library-linking"],
            },
          })
        ).to.be.rejectedWith(Error, "AllowlistAddressZero")
      })
    })
  })

  describe("upgradeProxy", () => {
    describe("when new contract fails upgradeability validation", () => {
      describe("when a variable was added before old variables", () => {
        it("should throw an error", async () => {
          await deployments.fixture()

          await expect(
            upgradeProxy("WalletRegistry", "WalletRegistryV2MisplacedNewSlot", {
              factoryOpts: {
                libraries: { EcdsaInactivity: EcdsaInactivity.address },
                signer: proxyAdminOwner,
              },
              proxyOpts: {
                constructorArgs: [AddressZero, AddressZero],
                unsafeAllow: ["external-library-linking"],
              },
            })
          ).to.be.rejectedWith(Error, "New storage layout is incompatible")
        })
      })

      describe("when a variable was removed", () => {
        it("should throw an error", async () => {
          await deployments.fixture()

          await expect(
            upgradeProxy("WalletRegistry", "WalletRegistryV2MissingSlot", {
              factoryOpts: {
                libraries: { EcdsaInactivity: EcdsaInactivity.address },
                signer: proxyAdminOwner,
              },
              proxyOpts: {
                constructorArgs: [AddressZero, AddressZero],
                unsafeAllow: ["external-library-linking"],
              },
            })
          ).to.be.rejectedWith(
            Error,
            "Deleted `_maliciousDkgResultNotificationRewardMultiplier`"
          )
        })
      })
    })

    describe("when a new contract is valid", () => {
      let tokenStaking: Contract
      let reimbursementPool: Contract
      let walletRegistryGovernance: Contract
      let walletRegistry: WalletRegistry
      let newWalletRegistry: WalletRegistryV2

      const newSortitionPoolAddress =
        "0x0000000000000000000000000000000000000101"
      const newRandomBeaconAddress =
        "0x0000000000000000000000000000000000000202"
      const newVarValue = "new variable set for new contract"

      before(async () => {
        await deployments.fixture()

        tokenStaking = await helpers.contracts.getContract("TokenStaking")
        reimbursementPool = await helpers.contracts.getContract(
          "ReimbursementPool"
        )
        walletRegistryGovernance = await helpers.contracts.getContract(
          "WalletRegistryGovernance"
        )
        walletRegistry = (await helpers.contracts.getContract(
          "WalletRegistry"
        )) as WalletRegistry & WalletRegistryV2

        expect(await walletRegistry.governance()).equal(
          walletRegistryGovernance.address
        )

        newWalletRegistry = (await upgradeProxy(
          "WalletRegistry",
          "WalletRegistryV2",
          {
            factoryOpts: {
              signer: proxyAdminOwner,
              libraries: { EcdsaInactivity: EcdsaInactivity.address },
            },
            proxyOpts: {
              constructorArgs: [newSortitionPoolAddress, tokenStaking.address],
              call: {
                fn: "initializeV2",
                args: [newRandomBeaconAddress, newVarValue],
              },
              unsafeAllow: ["external-library-linking"],
            },
          }
        )) as WalletRegistryV2
      })

      it("new instance should have the same address as the old one", async () => {
        expect(newWalletRegistry.address).equal(walletRegistry.address)
      })

      it("should not update governance", async () => {
        expect(await walletRegistry.governance()).equal(
          walletRegistryGovernance.address
        )
      })

      it("should use the new value of the immutable variable", async () => {
        expect(await walletRegistry.sortitionPool()).to.be.equal(
          newSortitionPoolAddress
        )
      })

      it("should reinitialize existing variable", async () => {
        expect(await walletRegistry.randomBeacon()).to.be.equal(
          newRandomBeaconAddress
        )
      })

      it("should initialize new variable", async () => {
        expect(await newWalletRegistry.newVar()).to.be.equal(newVarValue)
      })

      it("should not update already set variable", async () => {
        expect(await walletRegistry.reimbursementPool()).to.be.equal(
          reimbursementPool.address
        )
      })

      it("should not update parameters from library", async () => {
        expect((await walletRegistry.dkgParameters()).seedTimeout).to.be.equal(
          11_520
        )

        expect(await walletRegistry.minimumAuthorization()).to.be.equal(
          "40000000000000000000000"
        )
      })

      it("should revert when V1's initializer is called", async () => {
        await expect(
          newWalletRegistry.initialize(AddressZero, AddressZero, AddressZero)
        ).to.be.revertedWith("Initializable: contract is already initialized")
      })

      it("should revert for removed function", async () => {
        await expect(walletRegistry.notifySeedTimeout()).to.be.rejectedWith(
          Error,
          "Transaction reverted: function selector was not recognized and there's no fallback function"
        )
      })

      it("should execute updated function logic", async () => {
        await expect(walletRegistry.notifyDkgTimeout()).to.be.revertedWith(
          "nice try, but no"
        )
      })
    })

    describe("when a contract gets upgraded during DKG", () => {
      describe("when a wallet is already registered", () => {
        const expectedExistingWalletData = ecdsaData.group1
        const expectedNewWalletData = ecdsaData.group2
        const newWalletID = keccak256(expectedNewWalletData.publicKey)

        let walletRegistryV2: WalletRegistryV2

        let existingWalletID: string
        let existingWalletMembersHash: string
        let newWalletMembersHash: string

        before(async () => {
          // WalletRegistry V1
          const {
            walletRegistry: walletRegistryV1,
            randomBeacon,
            sortitionPool,
            staking,
            walletOwner,
          } = await walletRegistryFixture()

          // Create an existing wallet on Wallet Registry V1
          const existingWallet = await createNewWallet(
            walletRegistryV1,
            walletOwner.wallet,
            randomBeacon,
            expectedExistingWalletData.publicKey
          )
          existingWalletID = existingWallet.walletID
          existingWalletMembersHash = existingWallet.dkgResult.membersHash

          // Request new wallet (start DKG) on Wallet Registry V1
          const requestNewWalletTx = await walletRegistryV1
            .connect(walletOwner.wallet)
            .requestNewWallet()

          const relayEntry = ethers.utils.randomBytes(32)
          const dkgSeed = ethers.BigNumber.from(keccak256(relayEntry))

          // eslint-disable-next-line no-underscore-dangle
          await walletRegistryV1
            .connect(randomBeacon.wallet)
            .__beaconCallback(relayEntry, 0)

          // Submit DKG result on Wallet Registry V1
          const {
            dkgResult,
            submitter,
            transaction: submitDkgResultTx,
          } = await signAndSubmitCorrectDkgResult(
            walletRegistryV1,
            expectedNewWalletData.publicKey,
            dkgSeed,
            requestNewWalletTx.blockNumber,
            noMisbehaved
          )

          newWalletMembersHash = dkgResult.membersHash

          // Upgrade WalletRegistry from V1 to V2
          walletRegistryV2 = (await upgradeProxy(
            "WalletRegistry",
            "WalletRegistryV2",
            {
              factoryOpts: {
                signer: proxyAdminOwner,
                libraries: { EcdsaInactivity: EcdsaInactivity.address },
              },
              proxyOpts: {
                constructorArgs: [sortitionPool.address, staking.address],
                call: {
                  fn: "initializeV2",
                  args: [AddressZero, "new variable set for new contract"],
                },
                unsafeAllow: ["external-library-linking"],
              },
            }
          )) as WalletRegistryV2

          // Approve DKG result on Wallet Registry V2
          await mineBlocksTo(
            submitDkgResultTx.blockNumber +
              params.dkgResultChallengePeriodLength
          )

          await walletRegistryV1.connect(submitter).approveDkgResult(dkgResult)
        })

        it("keeps data of the existing wallet", async () => {
          const wallet = await walletRegistryV2.getWallet(existingWalletID)

          expect(wallet.publicKeyX).equal(expectedExistingWalletData.publicKeyX)
          expect(wallet.publicKeyY).equal(expectedExistingWalletData.publicKeyY)
          expect(wallet.membersIdsHash).equal(existingWalletMembersHash)
        })

        it("stores data of a new wallet", async () => {
          const wallet = await walletRegistryV2.getWallet(newWalletID)

          expect(wallet.publicKeyX).equal(expectedNewWalletData.publicKeyX)
          expect(wallet.publicKeyY).equal(expectedNewWalletData.publicKeyY)
          expect(wallet.membersIdsHash).equal(newWalletMembersHash)
        })
      })
    })

    // TODO: Test upgradeability of linked libraries
  })

  describe("when upgrade is called by the non-proxy-admin-owner", () => {
    it("should throw an error", async () => {
      await deployments.fixture()

      await expect(
        upgradeProxy("WalletRegistry", "WalletRegistryV2", {
          factoryOpts: {
            signer: (await helpers.signers.getNamedSigners()).deployer,
            libraries: { EcdsaInactivity: EcdsaInactivity.address },
          },
          proxyOpts: {
            constructorArgs: [AddressZero, AddressZero],
            unsafeAllow: ["external-library-linking"],
          },
        })
      ).to.be.rejectedWith(Error, "Ownable: caller is not the owner")
    })
  })
})

// TODO: Move to @keep-network/hardhat-helpers
export interface UpgradesUpgradeOptions {
  contractName?: string
  initializerArgs?: unknown[]
  factoryOpts?: FactoryOptions
  proxyOpts?: UpgradeProxyOptions
}

// TODO: Move to @keep-network/hardhat-helpers
async function upgradeProxy(
  currentContractName: string,
  newContractName: string,
  opts?: UpgradesUpgradeOptions
): Promise<Contract> {
  const currentContract = await deployments.get(currentContractName)

  const newContract = await ethers.getContractFactory(
    opts?.contractName || newContractName,
    opts?.factoryOpts
  )

  return upgrades.upgradeProxy(
    currentContract.address,
    newContract,
    opts?.proxyOpts
  )
}

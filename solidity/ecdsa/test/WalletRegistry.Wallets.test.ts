import { helpers, ethers } from "hardhat"
import { expect } from "chai"
import { formatBytes32String } from "ethers/lib/utils"

import { walletRegistryFixture } from "./fixtures"
import { createNewWallet } from "./utils/wallets"
import ecdsaData from "./data/ecdsa"
import { hashUint32Array } from "./utils/groups"

import type { Operator } from "./utils/operators"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type { ContractTransaction } from "ethers"
import type { DkgResult } from "./utils/dkg"
import type {
  IRandomBeacon,
  IWalletOwner,
  WalletRegistry,
  WalletRegistryStub,
} from "../typechain"
import type { FakeContract } from "@defi-wonderland/smock"

const { createSnapshot, restoreSnapshot } = helpers.snapshot

const validTestData = [
  {
    context: "with valid ECDSA key",
    publicKey: ecdsaData.group1.publicKey,
    expectedWalletID:
      "0xa6602e554b8cf7c23538fd040e4ff3520ec680e5e5ce9a075259e613a3e5aa79",
  },
  {
    context: "with leading zeros",
    publicKey:
      "0x000000440cc47779235ccb76d669590c2cd20c7e431f97e17a1093faf03291c473e661a208a8a565ca1e384059bd2ff7ff6886df081ff1229250099d388c83df",
    expectedWalletID:
      "0xd13b4fe9e69dde1520091494b27aab6c48a0058967551a25c525c843be472589",
  },
  {
    context: "with trailing zeros",
    publicKey:
      "0x9a0544440cc47779235ccb76d669590c2cd20c7e431f97e17a1093faf03291c473e661a208a8a565ca1e384059bd2ff7ff6886df081ff1229250099d00000000",
    expectedWalletID:
      "0x525e77a3052a07734c5736074a94b71dd9149650ef6a4c57dac696a3e287d03c",
  },
  {
    context: "with zeros in the middle",
    publicKey:
      "0x9a0544440cc47779235ccb76d669590c2cd20c7e431f97e17a1093faf0320000000061a208a8a565ca1e384059bd2ff7ff6886df081ff1229250099d388c83df",
    expectedWalletID:
      "0xa8b7226c57b544536f7bf805ef75c7b831488398da117644839f650c5be6cbe0",
  },
]

describe("WalletRegistry - Wallets", async () => {
  let walletRegistry: WalletRegistryStub & WalletRegistry
  let randomBeacon: FakeContract<IRandomBeacon>
  let walletOwner: FakeContract<IWalletOwner>
  let thirdParty: SignerWithAddress

  before("load test fixture", async () => {
    // eslint-disable-next-line @typescript-eslint/no-extra-semi
    ;({ walletRegistry, randomBeacon, walletOwner, thirdParty } =
      await walletRegistryFixture())
  })

  describe("approveDkgResult", async () => {
    context("with wallet not registered", async () => {
      context("with valid public key", async () => {
        validTestData.forEach((test) => {
          let walletID: string
          let dkgResult: DkgResult

          before("create a wallet", async () => {
            await createSnapshot()
            ;({ walletID, dkgResult } = await createNewWallet(
              walletRegistry,
              walletOwner.wallet,
              randomBeacon,
              test.publicKey
            ))
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should register wallet's details", async () => {
            const wallet = await walletRegistry.getWallet(walletID)

            expect(
              wallet.membersIdsHash,
              "unexpected members ids hash"
            ).to.be.equal(hashUint32Array(dkgResult.members))

            expect(wallet.publicKeyX, "unexpected public key X").to.be.equal(
              ethers.utils.hexDataSlice(test.publicKey, 0, 32)
            )
            expect(wallet.publicKeyY, "unexpected public key Y").to.be.equal(
              ethers.utils.hexDataSlice(test.publicKey, 32)
            )
          })

          it("should calculate wallet id", async () => {
            expect(walletID, "unexpected walletID").to.be.equal(
              test.expectedWalletID
            )
          })
        })
      })

      context("with invalid public key", async () => {
        const testData = [
          {
            context: "with too short public key",
            publicKey: ethers.utils.randomBytes(63),
            expectedError: "Invalid length of the public key",
          },
          {
            context: "with too long public key",
            publicKey: ethers.utils.randomBytes(65),
            expectedError: "Invalid length of the public key",
          },
        ]

        testData.forEach((test) => {
          context(test.context, async () => {
            before("create a wallet", async () => {
              await createSnapshot()
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should revert", async () => {
              await expect(
                createNewWallet(
                  walletRegistry,
                  walletOwner.wallet,
                  randomBeacon,
                  test.publicKey
                )
              ).to.be.revertedWith(test.expectedError)
            })
          })
        })
      })
    })

    context("with wallet registered", async () => {
      const walletPublicKey = ecdsaData.group1.publicKey

      before("create a wallet", async () => {
        await createSnapshot()

        await createNewWallet(
          walletRegistry,
          walletOwner.wallet,
          randomBeacon,
          walletPublicKey
        )
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("with the same public key", async () => {
        before(async () => {
          await createSnapshot()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should revert", async () => {
          await expect(
            createNewWallet(
              walletRegistry,
              walletOwner.wallet,
              randomBeacon,
              walletPublicKey
            )
          ).to.be.revertedWith(
            "Wallet with the given public key already exists"
          )
        })
      })

      context("with another public key", async () => {
        before(async () => {
          await createSnapshot()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should succeed", async () => {
          await expect(
            createNewWallet(
              walletRegistry,
              walletOwner.wallet,
              randomBeacon,
              ecdsaData.group2.publicKey
            )
          ).to.not.be.reverted
        })
      })
    })
  })

  describe("isWalletRegistered", async () => {
    context("with wallet not registered", async () => {
      it("should return false", async () => {
        await expect(
          await walletRegistry.isWalletRegistered(
            formatBytes32String("NON EXISTING")
          )
        ).to.be.false
      })
    })

    context("with wallet registered", async () => {
      let walletID: string

      before("create a wallet", async () => {
        await createSnapshot()
        ;({ walletID } = await createNewWallet(
          walletRegistry,
          walletOwner.wallet,
          randomBeacon
        ))
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should return true", async () => {
        await expect(await walletRegistry.isWalletRegistered(walletID)).to.be
          .true
      })
    })
  })

  describe("getWalletPublicKey", async () => {
    context("with wallet not registered", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.getWalletPublicKey(formatBytes32String("NON EXISTING"))
        ).to.be.revertedWith("Wallet with the given ID has not been registered")
      })
    })

    context("with wallet registered", async () => {
      validTestData.forEach((test) => {
        const walletPublicKey = test.publicKey
        let walletID: string

        before("create a wallet", async () => {
          await createSnapshot()
          ;({ walletID } = await createNewWallet(
            walletRegistry,
            walletOwner.wallet,
            randomBeacon,
            walletPublicKey
          ))
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should return uncompressed public key", async () => {
          const actualPublicKey = await walletRegistry.getWalletPublicKey(
            walletID
          )
          await expect(
            actualPublicKey,
            "returned public key doesn't match expected"
          ).to.be.equal(walletPublicKey)

          await expect(
            ethers.utils.arrayify(actualPublicKey),
            "returned public key is not 64-byte long"
          ).to.have.lengthOf(64)
        })
      })
    })
  })

  describe("closeWallet", async () => {
    let walletID: string

    before("create a wallet", async () => {
      await createSnapshot()
      ;({ walletID } = await createNewWallet(
        walletRegistry,
        walletOwner.wallet,
        randomBeacon
      ))
    })

    after(async () => {
      await restoreSnapshot()
    })

    context("when called by a third party", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(thirdParty).closeWallet(walletID)
        ).to.be.revertedWith("CallerNotWalletOwner")
      })
    })

    context("when caller is the wallet owner", () => {
      context("when wallet with the given ID is unknown", () => {
        it("should revert", async () => {
          const unknownWalletID: string = ethers.utils.keccak256(walletID)
          await expect(
            walletRegistry
              .connect(walletOwner.wallet)
              .closeWallet(unknownWalletID)
          ).to.be.revertedWith(
            "Wallet with the given ID has not been registered"
          )
        })
      })

      context("when wallet with the given ID is registered", () => {
        let tx: ContractTransaction

        before("close the wallet", async () => {
          await createSnapshot()

          tx = await walletRegistry
            .connect(walletOwner.wallet)
            .closeWallet(walletID)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should emit WalletClosed event", async () => {
          await expect(tx)
            .to.emit(walletRegistry, "WalletClosed")
            .withArgs(walletID)
        })

        it("should remove wallet from the registry", async () => {
          await expect(await walletRegistry.isWalletRegistered(walletID)).to.be
            .false
        })
      })

      context(
        "when the wallet with the given ID has already been closed",
        () => {
          before("close the wallet", async () => {
            await createSnapshot()
            await walletRegistry
              .connect(walletOwner.wallet)
              .closeWallet(walletID)
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should revert", async () => {
            await expect(
              walletRegistry.connect(walletOwner.wallet).closeWallet(walletID)
            ).to.be.revertedWith(
              "Wallet with the given ID has not been registered"
            )
          })
        }
      )
    })
  })

  describe("isWalletMember", () => {
    let walletID: string
    let walletMembersIDs: number[]
    let walletMembersAddresses: string[]

    before("create a wallet", async () => {
      await createSnapshot()

      let members: Operator[]
      ;({ walletID, members } = await createNewWallet(
        walletRegistry,
        walletOwner.wallet,
        randomBeacon
      ))

      walletMembersIDs = members.map((member) => member.id)
      walletMembersAddresses = members.map((member) => member.signer.address)
    })

    after(async () => {
      await restoreSnapshot()
    })

    context(
      "when the given operator address is an actual sortition pool operator",
      () => {
        context("when the passed wallet members identifiers are valid", () => {
          context("when the wallet member index is in correct range", () => {
            context(
              "when the given operator is the member of the wallet signing group at the given position",
              () => {
                it("should return true", async () => {
                  // eslint-disable-next-line @typescript-eslint/no-unused-expressions
                  expect(
                    await walletRegistry.isWalletMember(
                      walletID,
                      walletMembersIDs,
                      walletMembersAddresses[5],
                      6
                    )
                  ).to.be.true
                })
              }
            )

            context(
              "when the given operator is not the member of the wallet signing group at the given position",
              () => {
                it("should return false", async () => {
                  // eslint-disable-next-line @typescript-eslint/no-unused-expressions
                  expect(
                    await walletRegistry.isWalletMember(
                      walletID,
                      walletMembersIDs,
                      walletMembersAddresses[5],
                      7
                    )
                  ).to.be.false
                })
              }
            )
          })

          context("when the wallet member index is out of range", () => {
            context(
              "when the wallet member index is lesser than the minimum proper value",
              () => {
                it("should revert", async () => {
                  // Min proper value is `1`.
                  await expect(
                    walletRegistry.isWalletMember(
                      walletID,
                      walletMembersIDs,
                      walletMembersAddresses[0],
                      0
                    )
                  ).to.be.revertedWith("WalletMemberIndexOutOfRange")
                })
              }
            )

            context(
              "when the wallet member index is greater than the maximum proper value",
              () => {
                it("should revert", async () => {
                  // Max proper value is `walletMembersIDs.length`.
                  await expect(
                    walletRegistry.isWalletMember(
                      walletID,
                      walletMembersIDs,
                      walletMembersAddresses[0],
                      walletMembersIDs.length + 1
                    )
                  ).to.be.revertedWith("WalletMemberIndexOutOfRange")
                })
              }
            )
          })
        })

        context(
          "when the passed wallet members identifiers are invalid",
          () => {
            it("should revert", async () => {
              const corruptedWalletMembersIDs = walletMembersIDs.reverse()

              await expect(
                walletRegistry.isWalletMember(
                  walletID,
                  corruptedWalletMembersIDs,
                  walletMembersAddresses[0],
                  0
                )
              ).to.be.revertedWith("InvalidWalletMembersIdentifiers")
            })
          }
        )
      }
    )

    context(
      "when the given operator address is not an actual sortition pool operator",
      () => {
        it("should revert", async () => {
          // To test this scenario, we need an address that is not a
          // sortition pool operator for sure. The address of the wallet
          // registry itself seems to be a good candidate.
          const operator = walletRegistry.address

          await expect(
            walletRegistry.isWalletMember(
              walletID,
              walletMembersIDs,
              operator,
              0
            )
          ).to.be.revertedWith("NotSortitionPoolOperator")
        })
      }
    )
  })
})

/* eslint-disable no-underscore-dangle */
import { ethers, helpers, waffle } from "hardhat"
import { expect } from "chai"

import ecdsaData from "./data/ecdsa"
import { params, walletRegistryFixture } from "./fixtures"
import { createNewWallet } from "./utils/wallets"
import { signOperatorInactivityClaim } from "./utils/inactivity"
import { assertGasUsed } from "./helpers/gas"

import type { BigNumber, ContractTransaction } from "ethers"
import type { FakeContract } from "@defi-wonderland/smock"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type {
  SortitionPool,
  WalletRegistry,
  IWalletOwner,
  IRandomBeacon,
} from "../typechain"
import type { Operator, OperatorID } from "./utils/operators"

const { createSnapshot, restoreSnapshot } = helpers.snapshot
const { provider } = waffle

describe("WalletRegistry - Inactivity", () => {
  let walletRegistry: WalletRegistry
  let sortitionPool: SortitionPool
  let randomBeacon: FakeContract<IRandomBeacon>
  let walletOwner: FakeContract<IWalletOwner>

  let thirdParty: SignerWithAddress

  let members: Operator[]
  let membersIDs: OperatorID[]
  let walletID: string

  const walletPublicKey: string = ecdsaData.group1.publicKey
  const walletPublicKeyX: string = ecdsaData.group1.publicKeyX
  const walletPublicKeyY: string = ecdsaData.group1.publicKeyY

  // Use 49 element `inactiveMembersIndices` array to simulate the most gas
  // expensive real-world case. If group size is 100, the required threshold
  // is 51 so we assume 49 operators at most will be marked as ineligible
  // during a single `notifyOperatorInactivity` call.
  const subsequentInactiveMembersIndices = Array.from(
    Array(49),
    (_, i) => i + 1
  )
  const nonSubsequentInactiveMembersIndices = [2, 5, 7, 23, 56]
  const emptyMembersIndices = []

  const groupThreshold = 51

  const heartbeatFailed = true
  const noHeartbeatFailure = false

  before(async () => {
    // eslint-disable-next-line @typescript-eslint/no-extra-semi
    ;({ walletRegistry, sortitionPool, randomBeacon, walletOwner, thirdParty } =
      await walletRegistryFixture())
    ;({ members, walletID } = await createNewWallet(
      walletRegistry,
      walletOwner.wallet,
      randomBeacon,
      walletPublicKey
    ))

    membersIDs = members.map((member) => member.id)
  })

  describe("notifyOperatorInactivity", () => {
    context("when passed nonce is valid", () => {
      context("when wallet is known", () => {
        context("when inactive members indices are correct", () => {
          context("when signatures array is correct", () => {
            context("when signing members indices are correct", () => {
              context("when all signatures are correct", () => {
                context("when claim sender signed the claim", () => {
                  const assertNotifyInactivitySucceed = async (
                    inactiveMembersIndices: number[],
                    signaturesCount: number,
                    modifySignatures: (signatures: string) => string,
                    modifySigningMemberIndices: (
                      signingMemberIndices: number[]
                    ) => number[],
                    expectedGasUsed: number
                  ) => {
                    let tx: ContractTransaction
                    let initialNonce: BigNumber
                    let initClaimSenderBalance: BigNumber
                    let claimSender: SignerWithAddress

                    before(async () => {
                      await createSnapshot()

                      // Assume claim sender is the first signing member.
                      claimSender = members[0].signer

                      initialNonce = await walletRegistry.inactivityClaimNonce(
                        walletID
                      )

                      initClaimSenderBalance = await provider.getBalance(
                        claimSender.address
                      )

                      const { signatures, signingMembersIndices } =
                        await signOperatorInactivityClaim(
                          members,
                          0,
                          walletPublicKey,
                          noHeartbeatFailure,
                          inactiveMembersIndices,
                          signaturesCount
                        )

                      tx = await walletRegistry
                        .connect(claimSender)
                        .notifyOperatorInactivity(
                          {
                            walletID,
                            inactiveMembersIndices,
                            heartbeatFailed: noHeartbeatFailure,
                            signatures: modifySignatures(signatures),
                            signingMembersIndices: modifySigningMemberIndices(
                              signingMembersIndices
                            ),
                          },
                          0,
                          membersIDs
                        )
                    })

                    after(async () => {
                      await restoreSnapshot()
                    })

                    it("should refund ETH", async () => {
                      const postNotifyThirdPartyBalance =
                        await provider.getBalance(claimSender.address)
                      const diff = postNotifyThirdPartyBalance.sub(
                        initClaimSenderBalance
                      )

                      expect(diff).to.be.gt(0)
                      expect(diff).to.be.lt(
                        ethers.utils.parseUnits("2000000", "gwei") // 0,002 ETH
                      )
                    })

                    it(`should use close to ${expectedGasUsed} gas`, async () => {
                      await assertGasUsed(
                        tx,
                        expectedGasUsed,
                        ethers.BigNumber.from(expectedGasUsed)
                          .mul(5) // 5% delta
                          .div(100)
                          .toNumber()
                      )
                    })

                    it("should increment inactivity claim nonce for the group", async () => {
                      expect(
                        await walletRegistry.inactivityClaimNonce(walletID)
                      ).to.be.equal(initialNonce.add(1))
                    })

                    it("should emit InactivityClaimed event", async () => {
                      await expect(tx)
                        .to.emit(walletRegistry, "InactivityClaimed")
                        .withArgs(
                          walletID,
                          initialNonce.toNumber(),
                          claimSender.address
                        )
                    })

                    it("should ban sortition pool rewards for inactive operators", async () => {
                      const now = await helpers.time.lastBlockTime()
                      const expectedUntil =
                        now + params.sortitionPoolRewardsBanDuration

                      const expectedIneligibleMembersIDs =
                        inactiveMembersIndices.map((i) => membersIDs[i - 1])

                      await expect(tx)
                        .to.emit(sortitionPool, "IneligibleForRewards")
                        .withArgs(expectedIneligibleMembersIDs, expectedUntil)
                    })
                  }

                  context(
                    "when there are multiple subsequent inactive members indices",
                    async () => {
                      await assertNotifyInactivitySucceed(
                        subsequentInactiveMembersIndices,
                        groupThreshold,
                        (signatures) => signatures,
                        (signingMembersIndices) => signingMembersIndices,
                        1_210_000
                      )
                    }
                  )

                  context(
                    "when there is only one inactive member index",
                    async () => {
                      await assertNotifyInactivitySucceed(
                        [32],
                        groupThreshold,
                        (signatures) => signatures,
                        (signingMembersIndices) => signingMembersIndices,
                        840_000
                      )
                    }
                  )

                  context(
                    "when there are multiple non-subsequent inactive members indices",
                    async () => {
                      await assertNotifyInactivitySucceed(
                        nonSubsequentInactiveMembersIndices,
                        groupThreshold,
                        (signatures) => signatures,
                        (signingMembersIndices) => signingMembersIndices,
                        880_000
                      )
                    }
                  )

                  context(
                    "when there are multiple non-subsequent signing members indices",
                    async () => {
                      const newSigningMembersIndices = [
                        1, 5, 8, 11, 14, 15, 18, 20, 22, 24, 25, 27, 29, 30, 31,
                        33, 38, 39, 41, 42, 44, 47, 48, 49, 51, 53, 55, 56, 57,
                        59, 61, 62, 64, 65, 66, 67, 69, 71, 73, 75, 76, 78, 79,
                        80, 82, 83, 84, 86, 88, 90, 99,
                      ]

                      // we cut the first 2 characters to get rid of "0x" and
                      // then return signature on arbitrary position - each
                      // signature has 65 bytes so 130 characters
                      const getSignature = (signatures, index) =>
                        signatures
                          .slice(2)
                          .slice(130 * index, 130 * index + 130)

                      const modifySignatures = (signatures) => {
                        let newSignatures = "0x"

                        for (
                          let i = 0;
                          i < newSigningMembersIndices.length;
                          i++
                        ) {
                          const newSigningMemberIndex =
                            newSigningMembersIndices[i]
                          newSignatures += getSignature(
                            signatures,
                            newSigningMemberIndex - 1
                          )
                        }

                        return newSignatures
                      }

                      await assertNotifyInactivitySucceed(
                        subsequentInactiveMembersIndices,
                        // Make more signatures than needed to allow picking up
                        // arbitrary signatures.
                        100,
                        modifySignatures,
                        () => newSigningMembersIndices,
                        1_240_000
                      )
                    }
                  )

                  context("when heartbeat failed", () => {
                    before(async () => {
                      await createSnapshot()
                      // Assume claim sender is the first signing member.
                      const claimSender = members[0].signer

                      const { signatures, signingMembersIndices } =
                        await signOperatorInactivityClaim(
                          members,
                          0,
                          walletPublicKey,
                          heartbeatFailed,
                          subsequentInactiveMembersIndices,
                          groupThreshold
                        )
                      await walletRegistry
                        .connect(claimSender)
                        .notifyOperatorInactivity(
                          {
                            walletID,
                            inactiveMembersIndices:
                              subsequentInactiveMembersIndices,
                            heartbeatFailed,
                            signatures,
                            signingMembersIndices,
                          },
                          0,
                          membersIDs
                        )
                    })

                    after(async () => {
                      await restoreSnapshot()
                      walletOwner.__ecdsaWalletHeartbeatFailedCallback.reset()
                    })

                    it("should notify the wallet owner", async () => {
                      await expect(
                        walletOwner.__ecdsaWalletHeartbeatFailedCallback
                      ).to.be.calledWith(
                        walletID,
                        walletPublicKeyX,
                        walletPublicKeyY
                      )
                    })
                  })

                  context("when heartbeat did not fail", () => {
                    before(async () => {
                      await createSnapshot()
                      // Assume claim sender is the first signing member.
                      const claimSender = members[0].signer

                      const { signatures, signingMembersIndices } =
                        await signOperatorInactivityClaim(
                          members,
                          0,
                          walletPublicKey,
                          noHeartbeatFailure,
                          subsequentInactiveMembersIndices,
                          groupThreshold
                        )
                      await walletRegistry
                        .connect(claimSender)
                        .notifyOperatorInactivity(
                          {
                            walletID,
                            inactiveMembersIndices:
                              subsequentInactiveMembersIndices,
                            heartbeatFailed: noHeartbeatFailure,
                            signatures,
                            signingMembersIndices,
                          },
                          0,
                          membersIDs
                        )
                    })

                    after(async () => {
                      await restoreSnapshot()
                    })

                    it("should not notify the wallet owner", async () => {
                      await expect(
                        walletOwner.__ecdsaWalletHeartbeatFailedCallback
                      ).not.to.be.called
                    })
                  })
                })

                context(
                  "when claim sender did not sign the claim",
                  async () => {
                    it("should revert", async () => {
                      const { signatures, signingMembersIndices } =
                        await signOperatorInactivityClaim(
                          members,
                          0,
                          walletPublicKey,
                          noHeartbeatFailure,
                          subsequentInactiveMembersIndices,
                          groupThreshold
                        )

                      const claimSender = thirdParty

                      await expect(
                        walletRegistry
                          .connect(claimSender)
                          .notifyOperatorInactivity(
                            {
                              walletID,
                              inactiveMembersIndices:
                                subsequentInactiveMembersIndices,
                              heartbeatFailed: noHeartbeatFailure,
                              signatures,
                              signingMembersIndices,
                            },
                            0,
                            membersIDs
                          )
                      ).to.be.revertedWith("Sender must be claim signer")
                    })
                  }
                )
              })

              context("when one of the signatures is incorrect", () => {
                const assertInvalidSignature = async (invalidSignature) => {
                  // The 50 signers sign correct parameters. Invalid signature
                  // is expected to be provided by signer 51.
                  const { signatures, signingMembersIndices } =
                    await signOperatorInactivityClaim(
                      members,
                      0,
                      walletPublicKey,
                      noHeartbeatFailure,
                      subsequentInactiveMembersIndices,
                      groupThreshold - 1
                    )

                  await expect(
                    walletRegistry.notifyOperatorInactivity(
                      {
                        walletID,
                        inactiveMembersIndices:
                          subsequentInactiveMembersIndices,
                        heartbeatFailed: noHeartbeatFailure,
                        // Slice removes `0x` prefix from wrong signature.
                        signatures: signatures + invalidSignature.slice(2),
                        signingMembersIndices: [...signingMembersIndices, 51],
                      },
                      0,
                      membersIDs
                    )
                  ).to.be.revertedWith("Invalid signature")
                }

                context(
                  "when one of the signatures signed the wrong nonce",
                  () => {
                    it("should revert", async () => {
                      // Signer 51 signs wrong nonce.
                      const invalidSignature = (
                        await signOperatorInactivityClaim(
                          [members[50]],
                          1,
                          walletPublicKey,
                          noHeartbeatFailure,
                          subsequentInactiveMembersIndices,
                          1
                        )
                      ).signatures

                      await assertInvalidSignature(invalidSignature)
                    })
                  }
                )

                context(
                  "when one of the signatures signed the wrong group public key",
                  () => {
                    it("should revert", async () => {
                      // Signer 51 signs wrong group public key.
                      const invalidSignature = (
                        await signOperatorInactivityClaim(
                          [members[50]],
                          0,
                          "0x010203",
                          noHeartbeatFailure,
                          subsequentInactiveMembersIndices,
                          1
                        )
                      ).signatures

                      await assertInvalidSignature(invalidSignature)
                    })
                  }
                )

                context(
                  "when one of the signatures signed wrong heartbeat status",
                  () => {
                    it("should revert", async () => {
                      // Signer 51 signs invalid inactivity status.
                      const invalidSignature = (
                        await signOperatorInactivityClaim(
                          [members[50]],
                          0,
                          walletPublicKey,
                          heartbeatFailed,
                          subsequentInactiveMembersIndices,
                          1
                        )
                      ).signatures

                      await assertInvalidSignature(invalidSignature)
                    })
                  }
                )

                context(
                  "when one of the signatures signed the wrong inactive group members indices",
                  () => {
                    it("should revert", async () => {
                      // Signer 51 signs wrong inactive group members indices.
                      const invalidSignature = (
                        await signOperatorInactivityClaim(
                          [members[50]],
                          0,
                          walletPublicKey,
                          noHeartbeatFailure,
                          [1, 2, 3, 4, 5, 6, 7, 8],
                          1
                        )
                      ).signatures

                      await assertInvalidSignature(invalidSignature)
                    })
                  }
                )
              })
            })

            context("when signing members indices are incorrect", () => {
              context(
                "when signing members indices count is different than signatures count",
                () => {
                  it("should revert", async () => {
                    const { signatures, signingMembersIndices } =
                      await signOperatorInactivityClaim(
                        members,
                        0,
                        walletPublicKey,
                        noHeartbeatFailure,
                        subsequentInactiveMembersIndices,
                        groupThreshold
                      )

                    await expect(
                      walletRegistry.notifyOperatorInactivity(
                        {
                          walletID,
                          inactiveMembersIndices:
                            subsequentInactiveMembersIndices,
                          heartbeatFailed: noHeartbeatFailure,
                          signatures,
                          // Remove the first signing member index
                          signingMembersIndices: signingMembersIndices.slice(1),
                        },
                        0,
                        membersIDs
                      )
                    ).to.be.revertedWith("Unexpected signatures count")
                  })
                }
              )

              context("when first signing member index is zero", () => {
                it("should revert", async () => {
                  const { signatures, signingMembersIndices } =
                    await signOperatorInactivityClaim(
                      members,
                      0,
                      walletPublicKey,
                      noHeartbeatFailure,
                      subsequentInactiveMembersIndices,
                      groupThreshold
                    )

                  signingMembersIndices[0] = 0

                  await expect(
                    walletRegistry.notifyOperatorInactivity(
                      {
                        walletID,
                        inactiveMembersIndices:
                          subsequentInactiveMembersIndices,
                        heartbeatFailed: noHeartbeatFailure,
                        signatures,
                        signingMembersIndices,
                      },
                      0,
                      membersIDs
                    )
                  ).to.be.revertedWith("Corrupted members indices")
                })
              })

              context(
                "when last signing member index is bigger than group size",
                () => {
                  it("should revert", async () => {
                    const { signatures, signingMembersIndices } =
                      await signOperatorInactivityClaim(
                        members,
                        0,
                        walletPublicKey,
                        noHeartbeatFailure,
                        subsequentInactiveMembersIndices,
                        groupThreshold
                      )

                    signingMembersIndices[
                      signingMembersIndices.length - 1
                    ] = 101

                    await expect(
                      walletRegistry.notifyOperatorInactivity(
                        {
                          walletID,
                          inactiveMembersIndices:
                            subsequentInactiveMembersIndices,
                          heartbeatFailed: noHeartbeatFailure,
                          signatures,
                          signingMembersIndices,
                        },
                        0,
                        membersIDs
                      )
                    ).to.be.revertedWith("Corrupted members indices")
                  })
                }
              )

              context(
                "when signing members indices are not ordered in ascending order",
                () => {
                  it("should revert", async () => {
                    const { signatures, signingMembersIndices } =
                      await signOperatorInactivityClaim(
                        members,
                        0,
                        walletPublicKey,
                        noHeartbeatFailure,
                        subsequentInactiveMembersIndices,
                        groupThreshold
                      )

                    // eslint-disable-next-line prefer-destructuring
                    signingMembersIndices[10] = signingMembersIndices[11]

                    await expect(
                      walletRegistry.notifyOperatorInactivity(
                        {
                          walletID,
                          inactiveMembersIndices:
                            subsequentInactiveMembersIndices,
                          heartbeatFailed: noHeartbeatFailure,
                          signatures,
                          signingMembersIndices,
                        },
                        0,
                        membersIDs
                      )
                    ).to.be.revertedWith("Corrupted members indices")
                  })
                }
              )
            })
          })

          context("when signatures array is incorrect", () => {
            context("when signatures count is zero", () => {
              it("should revert", async () => {
                const signatures = "0x"

                await expect(
                  walletRegistry.notifyOperatorInactivity(
                    {
                      walletID,
                      inactiveMembersIndices: subsequentInactiveMembersIndices,
                      heartbeatFailed: noHeartbeatFailure,
                      signatures,
                      signingMembersIndices: emptyMembersIndices,
                    },
                    0,
                    membersIDs
                  )
                ).to.be.revertedWith("No signatures provided")
              })
            })

            context(
              "when signatures count is not divisible by signature byte size",
              () => {
                it("should revert", async () => {
                  const signatures = "0x010203"

                  await expect(
                    walletRegistry.notifyOperatorInactivity(
                      {
                        walletID,
                        inactiveMembersIndices:
                          subsequentInactiveMembersIndices,
                        heartbeatFailed: noHeartbeatFailure,
                        signatures,
                        signingMembersIndices: emptyMembersIndices,
                      },
                      0,
                      membersIDs
                    )
                  ).to.be.revertedWith("Malformed signatures array")
                })
              }
            )

            context(
              "when signatures count is different than signing members count",
              () => {
                it("should revert", async () => {
                  const { signatures, signingMembersIndices } =
                    await signOperatorInactivityClaim(
                      members,
                      0,
                      walletPublicKey,
                      noHeartbeatFailure,
                      subsequentInactiveMembersIndices,
                      groupThreshold
                    )

                  await expect(
                    walletRegistry.notifyOperatorInactivity(
                      {
                        walletID,
                        inactiveMembersIndices:
                          subsequentInactiveMembersIndices,
                        heartbeatFailed: noHeartbeatFailure,
                        // Remove the first signature to cause a mismatch with
                        // the signing members count.
                        signatures: `0x${signatures.slice(132)}`,
                        signingMembersIndices,
                      },
                      0,
                      membersIDs
                    )
                  ).to.be.revertedWith("Unexpected signatures count")
                })
              }
            )

            context(
              "when signatures count is less than group threshold",
              () => {
                it("should revert", async () => {
                  const { signatures, signingMembersIndices } =
                    await signOperatorInactivityClaim(
                      members,
                      0,
                      walletPublicKey,
                      noHeartbeatFailure,
                      subsequentInactiveMembersIndices,
                      // Provide one few signature
                      groupThreshold - 1
                    )

                  await expect(
                    walletRegistry.notifyOperatorInactivity(
                      {
                        walletID,
                        inactiveMembersIndices:
                          subsequentInactiveMembersIndices,
                        heartbeatFailed: noHeartbeatFailure,
                        signatures,
                        signingMembersIndices,
                      },
                      0,
                      membersIDs
                    )
                  ).to.be.revertedWith("Too few signatures")
                })
              }
            )

            context("when signatures count is bigger than group size", () => {
              it("should revert", async () => {
                const { signatures, signingMembersIndices } =
                  await signOperatorInactivityClaim(
                    members,
                    0,
                    walletPublicKey,
                    noHeartbeatFailure,
                    subsequentInactiveMembersIndices,
                    // All group signs.
                    members.length
                  )

                await expect(
                  walletRegistry.notifyOperatorInactivity(
                    {
                      walletID,
                      inactiveMembersIndices: subsequentInactiveMembersIndices,
                      heartbeatFailed: noHeartbeatFailure,
                      // Provide one more signature
                      // 2 to cut initial '0x' and 132 because signature length
                      // is 130 bytes, so 2+132 = 132
                      signatures: signatures + signatures.slice(2, 132),
                      signingMembersIndices: [
                        ...signingMembersIndices,
                        signingMembersIndices[0],
                      ],
                    },
                    0,
                    membersIDs
                  )
                ).to.be.revertedWith("Too many signatures")
              })
            })
          })
        })

        context("when inactive members indices are incorrect", () => {
          const assertInactiveMembersIndicesCorrupted = async (
            inactiveMembersIndices: number[]
          ) => {
            const { signatures, signingMembersIndices } =
              await signOperatorInactivityClaim(
                members,
                0,
                walletPublicKey,
                noHeartbeatFailure,
                inactiveMembersIndices,
                groupThreshold
              )

            await expect(
              walletRegistry.notifyOperatorInactivity(
                {
                  walletID,
                  inactiveMembersIndices,
                  heartbeatFailed: noHeartbeatFailure,
                  signatures,
                  signingMembersIndices,
                },
                0,
                membersIDs
              )
            ).to.be.revertedWith("Corrupted members indices")
          }

          context("when inactive members indices count is zero", () => {
            it("should revert", async () => {
              const inactiveMembersIndices = []

              await assertInactiveMembersIndicesCorrupted(
                inactiveMembersIndices
              )
            })
          })

          context(
            "when inactive members indices count is bigger than group size",
            () => {
              it("should revert", async () => {
                const inactiveMembersIndices = Array.from(
                  Array(101),
                  (_, i) => i + 1
                )

                await assertInactiveMembersIndicesCorrupted(
                  inactiveMembersIndices
                )
              })
            }
          )

          context("when first inactive member index is zero", () => {
            it("should revert", async () => {
              const inactiveMembersIndices = Array.from(
                Array(100),
                (_, i) => i + 1
              )
              inactiveMembersIndices[0] = 0

              await assertInactiveMembersIndicesCorrupted(
                inactiveMembersIndices
              )
            })
          })

          context(
            "when last inactive member index is bigger than group size",
            () => {
              it("should revert", async () => {
                const inactiveMembersIndices = Array.from(
                  Array(100),
                  (_, i) => i + 1
                )
                inactiveMembersIndices[inactiveMembersIndices.length - 1] = 101

                await assertInactiveMembersIndicesCorrupted(
                  inactiveMembersIndices
                )
              })
            }
          )

          context(
            "when inactive members indices are not ordered in ascending order",
            () => {
              it("should revert", async () => {
                const inactiveMembersIndices = Array.from(
                  Array(100),
                  (_, i) => i + 1
                )
                // eslint-disable-next-line prefer-destructuring
                inactiveMembersIndices[10] = inactiveMembersIndices[11]

                await assertInactiveMembersIndicesCorrupted(
                  inactiveMembersIndices
                )
              })
            }
          )
        })
      })

      context("when wallet public key is unknown", async () => {
        const unknownWalletPublicKey: string = ecdsaData.group2.publicKey

        it("should revert", async () => {
          const { signatures, signingMembersIndices } =
            await signOperatorInactivityClaim(
              members,
              0,
              unknownWalletPublicKey,
              noHeartbeatFailure,
              subsequentInactiveMembersIndices,
              groupThreshold
            )

          await expect(
            walletRegistry.notifyOperatorInactivity(
              {
                walletID,
                inactiveMembersIndices: subsequentInactiveMembersIndices,
                heartbeatFailed: noHeartbeatFailure,
                signatures,
                signingMembersIndices,
              },
              0,
              membersIDs
            )
          ).to.be.revertedWith("Invalid signature")
        })
      })

      context("when wallet ID is unknown", async () => {
        it("should revert", async () => {
          const unknownWalletID: string = ethers.utils.keccak256(walletID)

          const { signatures, signingMembersIndices } =
            await signOperatorInactivityClaim(
              members,
              0,
              walletPublicKey,
              noHeartbeatFailure,
              subsequentInactiveMembersIndices,
              groupThreshold
            )

          await expect(
            walletRegistry.notifyOperatorInactivity(
              {
                walletID: unknownWalletID,
                inactiveMembersIndices: subsequentInactiveMembersIndices,
                heartbeatFailed: noHeartbeatFailure,
                signatures,
                signingMembersIndices,
              },
              0,
              membersIDs
            )
          ).to.be.revertedWith(
            "Wallet with the given ID has not been registered"
          )
        })
      })

      context("when wallet has been closed", async () => {
        before("close the wallet", async () => {
          await createSnapshot()
          await walletRegistry.connect(walletOwner.wallet).closeWallet(walletID)
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should revert", async () => {
          const { signatures, signingMembersIndices } =
            await signOperatorInactivityClaim(
              members,
              0,
              walletPublicKey,
              noHeartbeatFailure,
              subsequentInactiveMembersIndices,
              groupThreshold
            )

          await expect(
            walletRegistry.notifyOperatorInactivity(
              {
                walletID,
                inactiveMembersIndices: subsequentInactiveMembersIndices,
                heartbeatFailed: noHeartbeatFailure,
                signatures,
                signingMembersIndices,
              },
              0,
              membersIDs
            )
          ).to.be.revertedWith(
            "Wallet with the given ID has not been registered"
          )
        })
      })
    })

    context("when passed nonce is invalid", () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.notifyOperatorInactivity(
            {
              walletID,
              inactiveMembersIndices: emptyMembersIndices,
              heartbeatFailed: noHeartbeatFailure,
              signatures: "0x",
              signingMembersIndices: emptyMembersIndices,
            },
            1,
            membersIDs
          ) // Initial nonce is `0`.
        ).to.be.revertedWithCustomError(walletRegistry, "InvalidNonce")
      })
    })

    context("when group members are invalid", () => {
      it("should revert", async () => {
        const invalidMembersId = [0, 1, 42]
        await expect(
          walletRegistry.notifyOperatorInactivity(
            {
              walletID,
              inactiveMembersIndices: emptyMembersIndices,
              heartbeatFailed: noHeartbeatFailure,
              signatures: "0x",
              signingMembersIndices: emptyMembersIndices,
            },
            0,
            invalidMembersId
          )
        ).to.be.revertedWithCustomError(walletRegistry, "InvalidGroupMembers")
      })
    })
  })

  describe("inactivityClaimNonce", () => {
    before(async () => {
      await createSnapshot()
    })

    after(async () => {
      await restoreSnapshot()
    })

    context("when inactivity was claimed", () => {
      it("should increment continuously", async () => {
        // Assume claim sender is the first signing member.
        const claimSender = members[0].signer

        // Sign the first inactivity claim for the group, make sure the nonce
        // increments to 1.
        let { signatures, signingMembersIndices } =
          await signOperatorInactivityClaim(
            members,
            0,
            walletPublicKey,
            noHeartbeatFailure,
            subsequentInactiveMembersIndices,
            groupThreshold
          )
        await walletRegistry.connect(claimSender).notifyOperatorInactivity(
          {
            walletID,
            inactiveMembersIndices: subsequentInactiveMembersIndices,
            heartbeatFailed: noHeartbeatFailure,
            signatures,
            signingMembersIndices,
          },
          0,
          membersIDs
        )

        expect(await walletRegistry.inactivityClaimNonce(walletID)).to.be.equal(
          1
        )

        // Sign the second inactivity claim for the group, make sure the nonce
        // increments to 2.
        ;({ signatures, signingMembersIndices } =
          await signOperatorInactivityClaim(
            members,
            1,
            walletPublicKey,
            noHeartbeatFailure,
            subsequentInactiveMembersIndices,
            groupThreshold
          ))
        await walletRegistry.connect(claimSender).notifyOperatorInactivity(
          {
            walletID,
            inactiveMembersIndices: subsequentInactiveMembersIndices,
            heartbeatFailed: noHeartbeatFailure,
            signatures,
            signingMembersIndices,
          },
          1,
          membersIDs
        )

        expect(await walletRegistry.inactivityClaimNonce(walletID)).to.be.equal(
          2
        )
      })

      // In case a new wallet with the same ID (public key) is registered, we
      // need to make sure old inactivity claims do not work.
      // Wallet owner may decide to revert in __ecdsaWalletCreatedCallback in
      // case it is important to have the public keys unique but from the
      // perspective of WalletRegistry this is not strictly required as long as
      // inactivity claim nonces are not cleared up upon closing the wallet.
      context("when wallet has been closed", () => {
        before(async () => {
          await walletRegistry.connect(walletOwner.wallet).closeWallet(walletID)
        })

        it("should remain unchanged", async () => {
          expect(
            await walletRegistry.inactivityClaimNonce(walletID)
          ).to.be.equal(2)
        })
      })
    })
  })
})

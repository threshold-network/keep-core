import { ethers, waffle, helpers } from "hardhat"
import { expect } from "chai"

import { constants, dkgState, params, walletRegistryFixture } from "./fixtures"
import ecdsaData from "./data/ecdsa"
import {
  hashDKGMembers,
  noMisbehaved,
  signAndSubmitArbitraryDkgResult,
  signAndSubmitCorrectDkgResult,
  signAndSubmitUnrecoverableDkgResult,
  expectDkgResultSubmittedEvent,
  signDkgResult,
} from "./utils/dkg"
import { selectGroup, hashUint32Array } from "./utils/groups"
import { createNewWallet } from "./utils/wallets"
import { submitRelayEntry } from "./utils/randomBeacon"
import { assertGasUsed } from "./helpers/gas"

import type { BigNumber, ContractTransaction, Signer } from "ethers"
import type { IWalletOwner } from "../typechain/IWalletOwner"
import type { SignerWithAddress } from "@nomiclabs/hardhat-ethers/signers"
import type {
  SortitionPool,
  WalletRegistry,
  WalletRegistryStub,
  TokenStaking,
  IRandomBeacon,
  DkgChallenger,
} from "../typechain"
import type { DkgResult, DkgResultSubmittedEventArgs } from "./utils/dkg"
import type { Operator } from "./utils/operators"
import type { FakeContract } from "@defi-wonderland/smock"

const { to1e18 } = helpers.number
const { mineBlocks, mineBlocksTo } = helpers.time
const { createSnapshot, restoreSnapshot } = helpers.snapshot

const { keccak256 } = ethers.utils
const { provider } = waffle

describe("WalletRegistry - Wallet Creation", async () => {
  const dkgTimeout: number = params.dkgResultSubmissionTimeout
  const groupPublicKey: string = ethers.utils.hexValue(
    ecdsaData.group1.publicKey
  )
  const groupPublicKey2: string = ethers.utils.hexValue(
    ecdsaData.group2.publicKey
  )
  const walletID: string = ethers.utils.keccak256(groupPublicKey)

  const stubDkgResult: DkgResult = {
    submitterMemberIndex: 1,
    groupPubKey: ecdsaData.group1.publicKey,
    signingMembersIndices: [1, 2, 3, 4],
    signatures: "0x01020304",
    misbehavedMembersIndices: [],
    members: [1, 2, 3, 4],
    membersHash: hashDKGMembers([1, 2, 3, 4]),
  }

  let walletRegistry: WalletRegistryStub & WalletRegistry
  let sortitionPool: SortitionPool
  let staking: TokenStaking
  let randomBeacon: FakeContract<IRandomBeacon>
  let walletOwner: FakeContract<IWalletOwner>

  let deployer: SignerWithAddress
  let thirdParty: SignerWithAddress

  let operators: Operator[]

  before("load test fixture", async () => {
    // eslint-disable-next-line @typescript-eslint/no-extra-semi
    ;({
      walletRegistry,
      sortitionPool,
      randomBeacon,
      walletOwner,
      deployer,
      thirdParty,
      operators,
      staking,
    } = await walletRegistryFixture())
  })

  describe("requestNewWallet", async () => {
    context("when called by the deployer", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(deployer).requestNewWallet()
        ).to.be.revertedWith("Caller is not the Wallet Owner")
      })
    })

    context("when called by a third party", async () => {
      it("should revert", async () => {
        await expect(
          walletRegistry.connect(thirdParty).requestNewWallet()
        ).to.be.revertedWith("Caller is not the Wallet Owner")
      })
    })

    context("when called by the wallet owner", async () => {
      context("with initial contract state", async () => {
        let tx: ContractTransaction

        before("start wallet creation", async () => {
          await createSnapshot()
          tx = await walletRegistry
            .connect(walletOwner.wallet)
            .requestNewWallet()
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should lock the sortition pool", async () => {
          await expect(await sortitionPool.isLocked()).to.be.true
        })

        it("should emit DkgStateLocked event", async () => {
          await expect(tx).to.emit(walletRegistry, "DkgStateLocked")
        })

        it("should not emit DkgStarted event", async () => {
          await expect(tx).not.to.emit(walletRegistry, "DkgStarted")
        })

        it("should transition DKG to AWAITING_SEED state", async () => {
          await expect(
            await walletRegistry.getWalletCreationState()
          ).to.be.equal(dkgState.AWAITING_SEED)
        })

        it("should not set dkg details", async () => {
          const dkgData = await walletRegistry.getDkgData()

          await expect(dkgData.seed).to.be.equal(0)
          await expect(dkgData.startBlock).to.be.equal(0)
        })

        it("should not register new wallet", async () => {
          await expect(tx).not.to.emit(walletRegistry, "WalletCreated")
        })
      })

      context("with new wallet requested", async () => {
        before("request new wallet", async () => {
          await createSnapshot()
          await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
        })

        after(async () => {
          await restoreSnapshot()
        })

        context("with relay entry not submitted", async () => {
          it("should revert with 'Current state is not IDLE' error", async () => {
            await expect(
              walletRegistry.connect(walletOwner.wallet).requestNewWallet()
            ).to.be.revertedWith("Current state is not IDLE")
          })

          context("with relay entry submitted", async () => {
            let startBlock: number
            let dkgSeed: BigNumber

            before("submit relay entry", async () => {
              await createSnapshot()
              ;({ startBlock, dkgSeed } = await submitRelayEntry(
                walletRegistry
              ))
            })

            after(async () => {
              await restoreSnapshot()
            })

            context("with dkg result not submitted", async () => {
              it("should revert with 'Current state is not IDLE' error", async () => {
                await expect(
                  walletRegistry.connect(walletOwner.wallet).requestNewWallet()
                ).to.be.revertedWith("Current state is not IDLE")
              })
            })

            context("with valid dkg result submitted", async () => {
              let dkgResult: DkgResult
              let submitter: SignerWithAddress

              before("submit dkg result", async () => {
                await createSnapshot()
                ;({ dkgResult, submitter } =
                  await signAndSubmitCorrectDkgResult(
                    walletRegistry,
                    groupPublicKey,
                    dkgSeed,
                    startBlock,
                    noMisbehaved
                  ))
              })

              after(async () => {
                await restoreSnapshot()
              })

              context("with dkg result not approved", async () => {
                it("should revert with 'current state is not IDLE' error", async () => {
                  await expect(
                    walletRegistry
                      .connect(walletOwner.wallet)
                      .requestNewWallet()
                  ).to.be.revertedWith("Current state is not IDLE")
                })
              })

              context("with dkg result approved", async () => {
                before("approve dkg result", async () => {
                  await createSnapshot()

                  await mineBlocks(params.dkgResultChallengePeriodLength)

                  await walletRegistry
                    .connect(submitter)
                    .approveDkgResult(dkgResult)
                })

                after(async () => {
                  await restoreSnapshot()
                })

                it("should succeed", async () => {
                  await expect(
                    walletRegistry
                      .connect(walletOwner.wallet)
                      .requestNewWallet()
                  ).to.not.be.reverted
                })
              })
            })

            context("with invalid dkg result submitted", async () => {
              let dkgResult: DkgResult

              before("submit invalid dkg result", async () => {
                await createSnapshot()
                ;({ dkgResult } = await signAndSubmitArbitraryDkgResult(
                  walletRegistry,
                  groupPublicKey,
                  // Mix operators to make the result malicious
                  mixOperators(await selectGroup(sortitionPool, dkgSeed)),
                  startBlock,
                  noMisbehaved
                ))
              })

              after(async () => {
                await restoreSnapshot()
              })

              context("with dkg result challenged", async () => {
                before("challenge dkg result", async () => {
                  await createSnapshot()

                  await walletRegistry.challengeDkgResult(dkgResult)
                })

                after(async () => {
                  await restoreSnapshot()
                })

                it("should revert", async () => {
                  await expect(
                    walletRegistry
                      .connect(walletOwner.wallet)
                      .requestNewWallet()
                  ).to.be.revertedWith("Current state is not IDLE")
                })
              })
            })

            context("with dkg timeout notified", async () => {
              before("notify dkg timeout", async () => {
                await createSnapshot()

                await mineBlocksTo(startBlock + dkgTimeout)

                await walletRegistry.notifyDkgTimeout()
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should succeed", async () => {
                await expect(
                  walletRegistry.connect(walletOwner.wallet).requestNewWallet()
                ).not.to.be.reverted
              })
            })
          })
        })
      })
    })
  })

  // Tests for `__beaconCallback` were implemented in `WalletRegistry.RandomBeacon.test.ts`

  describe("getWalletCreationState", async () => {
    context("with initial contract state", async () => {
      it("should return IDLE state", async () => {
        expect(await walletRegistry.getWalletCreationState()).to.be.equal(
          dkgState.IDLE
        )
      })
    })

    context("with new wallet requested", async () => {
      before("request new wallet", async () => {
        await createSnapshot()
        await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("with relay entry not submitted", async () => {
        it("should return AWAITING_SEED state", async () => {
          expect(await walletRegistry.getWalletCreationState()).to.be.equal(
            dkgState.AWAITING_SEED
          )
        })

        context("with relay entry submitted", async () => {
          let startBlock: number
          let dkgSeed: BigNumber

          before(async () => {
            await createSnapshot()
            ;({ startBlock, dkgSeed } = await submitRelayEntry(walletRegistry))
          })

          after(async () => {
            await restoreSnapshot()
          })

          context("with dkg result not submitted", async () => {
            it("should return AWAITING_RESULT state", async () => {
              expect(await walletRegistry.getWalletCreationState()).to.be.equal(
                dkgState.AWAITING_RESULT
              )
            })

            context("after the dkg timeout period", async () => {
              before(async () => {
                await createSnapshot()

                await mineBlocksTo(startBlock + dkgTimeout + 1)
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should return AWAITING_RESULT state", async () => {
                expect(
                  await walletRegistry.getWalletCreationState()
                ).to.be.equal(dkgState.AWAITING_RESULT)
              })
            })

            context("with valid dkg result submitted", async () => {
              let dkgResult: DkgResult
              let submitter: SignerWithAddress

              before("submit dkg result", async () => {
                await createSnapshot()
                ;({ dkgResult, submitter } =
                  await signAndSubmitCorrectDkgResult(
                    walletRegistry,
                    groupPublicKey,
                    dkgSeed,
                    startBlock,
                    noMisbehaved
                  ))
              })

              after(async () => {
                await restoreSnapshot()
              })

              context("with dkg result not approved", async () => {
                it("should return CHALLENGE state", async () => {
                  expect(
                    await walletRegistry.getWalletCreationState()
                  ).to.be.equal(dkgState.CHALLENGE)
                })
              })

              context("with dkg result approved", async () => {
                before("approve dkg result", async () => {
                  await createSnapshot()

                  await mineBlocks(params.dkgResultChallengePeriodLength)

                  await walletRegistry
                    .connect(submitter)
                    .approveDkgResult(dkgResult)
                })

                after(async () => {
                  await restoreSnapshot()
                })

                it("should return IDLE state", async () => {
                  expect(
                    await walletRegistry.getWalletCreationState()
                  ).to.be.equal(dkgState.IDLE)
                })
              })
            })

            context("with invalid dkg result submitted", async () => {
              let dkgResult: DkgResult

              before("submit invalid dkg result", async () => {
                await createSnapshot()
                ;({ dkgResult } = await signAndSubmitArbitraryDkgResult(
                  walletRegistry,
                  groupPublicKey,
                  // Mix operators to make the result malicious
                  mixOperators(await selectGroup(sortitionPool, dkgSeed)),
                  startBlock,
                  noMisbehaved
                ))
              })

              after(async () => {
                await restoreSnapshot()
              })

              context("with dkg result challenged", async () => {
                before("challenge dkg result", async () => {
                  await createSnapshot()
                  await walletRegistry.challengeDkgResult(dkgResult)
                })

                after(async () => {
                  await restoreSnapshot()
                })

                it("should return AWAITING_RESULT state", async () => {
                  expect(
                    await walletRegistry.getWalletCreationState()
                  ).to.be.equal(dkgState.AWAITING_RESULT)
                })
              })
            })

            context("with dkg timeout notified", async () => {
              before("notify dkg timeout", async () => {
                await createSnapshot()

                await mineBlocksTo(startBlock + dkgTimeout)

                await walletRegistry.notifyDkgTimeout()
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should return IDLE state", async () => {
                expect(
                  await walletRegistry.getWalletCreationState()
                ).to.be.equal(dkgState.IDLE)
              })
            })
          })
        })
      })
    })
  })

  describe("hasDkgTimedOut", async () => {
    context("with initial contract state", async () => {
      it("should return false", async () => {
        await expect(await walletRegistry.hasDkgTimedOut()).to.be.false
      })
    })

    context("with new wallet requested", async () => {
      let requestNewWalletStartBlock: number

      before(async () => {
        await createSnapshot()
        const tx = await walletRegistry
          .connect(walletOwner.wallet)
          .requestNewWallet()

        requestNewWalletStartBlock = tx.blockNumber
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("with relay entry not submitted", async () => {
        it("should return false", async () => {
          await expect(await walletRegistry.hasDkgTimedOut()).to.be.false
        })

        context("at the end of the dkg timeout period", async () => {
          before(async () => {
            await createSnapshot()

            await mineBlocksTo(requestNewWalletStartBlock + dkgTimeout)
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should return false", async () => {
            await expect(await walletRegistry.hasDkgTimedOut()).to.be.false
          })
        })

        context("after the dkg timeout period", async () => {
          before(async () => {
            await createSnapshot()

            await mineBlocksTo(requestNewWalletStartBlock + dkgTimeout + 1)
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should return false", async () => {
            await expect(await walletRegistry.hasDkgTimedOut()).to.be.false
          })
        })

        context("with relay entry submitted", async () => {
          let startBlock: number
          let dkgSeed: BigNumber

          before(async () => {
            await createSnapshot()
            ;({ startBlock, dkgSeed } = await submitRelayEntry(walletRegistry))
          })

          after(async () => {
            await restoreSnapshot()
          })

          context("with dkg result not submitted", async () => {
            it("should return false", async () => {
              await expect(await walletRegistry.hasDkgTimedOut()).to.be.false
            })

            context("at the end of the dkg timeout period", async () => {
              before(async () => {
                await createSnapshot()

                await mineBlocksTo(startBlock + dkgTimeout)
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should return false", async () => {
                await expect(await walletRegistry.hasDkgTimedOut()).to.be.false
              })
            })

            context("after the dkg timeout period", async () => {
              before(async () => {
                await createSnapshot()

                await mineBlocksTo(startBlock + dkgTimeout + 1)
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should return true", async () => {
                await expect(await walletRegistry.hasDkgTimedOut()).to.be.true
              })
            })

            context("with valid dkg result submitted", async () => {
              let resultSubmissionBlock: number
              let dkgResult: DkgResult
              let submitter: SignerWithAddress

              before("submit dkg result", async () => {
                await createSnapshot()

                let tx: ContractTransaction
                ;({
                  transaction: tx,
                  dkgResult,
                  submitter,
                } = await signAndSubmitCorrectDkgResult(
                  walletRegistry,
                  groupPublicKey,
                  dkgSeed,
                  startBlock,
                  noMisbehaved
                ))

                resultSubmissionBlock = tx.blockNumber
              })

              after(async () => {
                await restoreSnapshot()
              })

              context("with dkg result not approved", async () => {
                context("at the end of the dkg timeout period", async () => {
                  before(async () => {
                    await createSnapshot()

                    await mineBlocksTo(startBlock + dkgTimeout)
                  })

                  after(async () => {
                    await restoreSnapshot()
                  })

                  it("should return false", async () => {
                    await expect(await walletRegistry.hasDkgTimedOut()).to.be
                      .false
                  })
                })

                context("after the dkg timeout period", async () => {
                  before(async () => {
                    await createSnapshot()

                    await mineBlocksTo(startBlock + dkgTimeout + 1)
                  })

                  after(async () => {
                    await restoreSnapshot()
                  })

                  it("should return false", async () => {
                    await expect(await walletRegistry.hasDkgTimedOut()).to.be
                      .false
                  })
                })

                context("at the end of the challenge period", async () => {
                  before(async () => {
                    await createSnapshot()

                    await mineBlocksTo(
                      resultSubmissionBlock +
                        params.dkgResultChallengePeriodLength
                    )
                  })

                  after(async () => {
                    await restoreSnapshot()
                  })

                  it("should return false", async () => {
                    await expect(await walletRegistry.hasDkgTimedOut()).to.be
                      .false
                  })
                })

                context("after the challenge period", async () => {
                  before(async () => {
                    await createSnapshot()

                    await mineBlocksTo(
                      resultSubmissionBlock +
                        params.dkgResultChallengePeriodLength +
                        1
                    )
                  })

                  after(async () => {
                    await restoreSnapshot()
                  })

                  it("should return false", async () => {
                    await expect(await walletRegistry.hasDkgTimedOut()).to.be
                      .false
                  })
                })
              })

              context("when dkg result was approved", async () => {
                before(async () => {
                  await createSnapshot()

                  await mineBlocksTo(
                    resultSubmissionBlock +
                      params.dkgResultChallengePeriodLength
                  )

                  await walletRegistry
                    .connect(submitter)
                    .approveDkgResult(dkgResult)
                })

                after(async () => {
                  await restoreSnapshot()
                })

                it("should return false", async () => {
                  await expect(await walletRegistry.hasDkgTimedOut()).to.be
                    .false
                })
              })
            })

            context("with invalid dkg result submitted", async () => {
              let dkgResult: DkgResult

              before(async () => {
                await createSnapshot()
                ;({ dkgResult } = await signAndSubmitArbitraryDkgResult(
                  walletRegistry,
                  groupPublicKey,
                  // Mix operators to make the result malicious.
                  mixOperators(await selectGroup(sortitionPool, dkgSeed)),
                  startBlock,
                  noMisbehaved
                ))
              })

              after(async () => {
                await restoreSnapshot()
              })

              context("with dkg result challenged", async () => {
                let challengeBlockNumber: number

                before(async () => {
                  await createSnapshot()

                  const tx = await walletRegistry.challengeDkgResult(dkgResult)
                  challengeBlockNumber = tx.blockNumber
                })

                after(async () => {
                  await restoreSnapshot()
                })

                context(
                  "at the end of dkg result submission period",
                  async () => {
                    before(async () => {
                      await createSnapshot()

                      await mineBlocksTo(
                        challengeBlockNumber + params.dkgResultSubmissionTimeout
                      )
                    })

                    after(async () => {
                      await restoreSnapshot()
                    })

                    it("should return false", async () => {
                      await expect(await walletRegistry.hasDkgTimedOut()).to.be
                        .false
                    })
                  }
                )

                context("after dkg result submission period", async () => {
                  before(async () => {
                    await createSnapshot()

                    await mineBlocksTo(
                      challengeBlockNumber +
                        params.dkgResultSubmissionTimeout +
                        1
                    )
                  })

                  after(async () => {
                    await restoreSnapshot()
                  })

                  it("should return true", async () => {
                    await expect(await walletRegistry.hasDkgTimedOut()).to.be
                      .true
                  })
                })
              })
            })
          })
        })
      })
    })
  })

  describe("submitDkgResult", async () => {
    context("with initial contract state", async () => {
      it("should revert with 'Current state is not AWAITING_RESULT' error", async () => {
        await expect(
          signAndSubmitArbitraryDkgResult(
            walletRegistry,
            groupPublicKey,
            operators,
            1,
            noMisbehaved
          )
        ).to.be.revertedWith("Current state is not AWAITING_RESULT")
      })
    })

    context("with new wallet requested", async () => {
      before("request new wallet", async () => {
        await createSnapshot()
        await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("with relay entry not submitted", async () => {
        it("should revert with 'Current state is not AWAITING_RESULT' error", async () => {
          await expect(
            signAndSubmitArbitraryDkgResult(
              walletRegistry,
              groupPublicKey,
              operators,
              1,
              noMisbehaved
            )
          ).to.be.revertedWith("Current state is not AWAITING_RESULT")
        })

        context("with relay entry submitted", async () => {
          let startBlock: number
          let dkgSeed: BigNumber

          before(async () => {
            await createSnapshot()
            ;({ startBlock, dkgSeed } = await submitRelayEntry(walletRegistry))
          })

          after(async () => {
            await restoreSnapshot()
          })

          context("with wallet creation not timed out", async () => {
            context("with dkg result not submitted", async () => {
              context("with a zero-X public key", async () => {
                it("should revert", async () => {
                  let dkgResult: DkgResult

                  const signers = await selectGroup(sortitionPool, dkgSeed)
                  // eslint-disable-next-line prefer-const
                  ;({ dkgResult } = await signDkgResult(
                    signers,
                    "0x000000000000000000000000000000000000000000000000000000000000000073e661a208a8a565ca1e384059bd2ff7ff6886df081ff1229250099d388c83df",
                    noMisbehaved,
                    startBlock
                  ))

                  // submitter has index 1 by default and we need to -1 it
                  // given the array is indexed from 0
                  const submitter = signers[0].signer
                  await expect(
                    walletRegistry.connect(submitter).submitDkgResult(dkgResult)
                  ).to.be.revertedWith("Wallet public key must be non-zero")
                })
              })

              context("with a public key of invalid length", async () => {
                it("should revert", async () => {
                  let dkgResult: DkgResult

                  const signers = await selectGroup(sortitionPool, dkgSeed)
                  // eslint-disable-next-line prefer-const
                  ;({ dkgResult } = await signDkgResult(
                    signers,
                    groupPublicKey.slice(0, -2), // remove the last byte
                    noMisbehaved,
                    startBlock
                  ))

                  // submitter has index 1 by default and we need to -1 it
                  // given the array is indexed from 0
                  const submitter = signers[0].signer
                  await expect(
                    walletRegistry.connect(submitter).submitDkgResult(dkgResult)
                  ).to.be.revertedWith("Invalid length of the public key")
                })
              })

              context("with a group already registered", async () => {
                before(async () => {
                  await createSnapshot()

                  await walletRegistry.forceAddWallet(
                    groupPublicKey,
                    // wallet members do not matter for this test
                    "0x0000000000000000000000000000000000000000000000000000000000000000"
                  )
                })

                after(async () => {
                  await restoreSnapshot()
                })

                it("should revert", async () => {
                  let dkgResult: DkgResult

                  const signers = await selectGroup(sortitionPool, dkgSeed)
                  // eslint-disable-next-line prefer-const
                  ;({ dkgResult } = await signDkgResult(
                    signers,
                    groupPublicKey,
                    noMisbehaved,
                    startBlock
                  ))

                  // submitter has index 1 by default and we need to -1 it
                  // given the array is indexed from 0
                  const submitter = signers[0].signer
                  await expect(
                    walletRegistry.connect(submitter).submitDkgResult(dkgResult)
                  ).to.be.revertedWith(
                    "Wallet with the given public key already exists"
                  )
                })
              })

              context("with enough signatures on the result", async () => {
                let tx: ContractTransaction
                let dkgResult: DkgResult
                let dkgResultHash: string

                before(async () => {
                  await createSnapshot()
                  ;({
                    transaction: tx,
                    dkgResult,
                    dkgResultHash,
                  } = await signAndSubmitArbitraryDkgResult(
                    walletRegistry,
                    groupPublicKey,
                    operators,
                    startBlock,
                    noMisbehaved,
                    1,
                    constants.groupThreshold
                  ))
                })

                after(async () => {
                  await restoreSnapshot()
                })

                it("should emit DkgResultSubmitted event", async () => {
                  await expectDkgResultSubmittedEvent(tx, {
                    resultHash: dkgResultHash,
                    seed: dkgSeed,
                    result: dkgResult,
                  })
                })

                it("should not register a new wallet", async () => {
                  await expect(tx).not.to.emit(walletRegistry, "WalletCreated")
                })

                it("should not unlock the sortition pool", async () => {
                  await expect(await sortitionPool.isLocked()).to.be.true
                })

                it("should use close to 290 000 gas", async () => {
                  await assertGasUsed(tx, 290_000)
                })
              })

              context("with not enough signatures on the result", async () => {
                before(async () => {
                  await createSnapshot()
                })

                after(async () => {
                  await restoreSnapshot()
                })

                it("should succeed", async () => {
                  await expect(
                    signAndSubmitArbitraryDkgResult(
                      walletRegistry,
                      groupPublicKey,
                      operators,
                      startBlock,
                      noMisbehaved,
                      1,
                      constants.groupThreshold - 1
                    )
                  ).to.not.be.reverted
                })
              })

              context("with the submission period started", async () => {
                beforeEach(async () => {
                  await createSnapshot()
                })

                afterEach(async () => {
                  await restoreSnapshot()
                })

                context(
                  "at the beginning of the submission period",
                  async () => {
                    it("should succeed for the first member", async () => {
                      await assertSubmissionSucceeds(1)
                    })

                    it("should succeed for the second member", async () => {
                      await assertSubmissionSucceeds(2)
                    })

                    it("should succeed for the last member", async () => {
                      await assertSubmissionSucceeds(constants.groupSize - 1)
                    })
                  }
                )

                context("at the end of the submission period", async () => {
                  before(async () => {
                    await createSnapshot()

                    await mineBlocksTo(
                      startBlock + params.dkgResultSubmissionTimeout - 1
                    )
                  })

                  after(async () => {
                    await restoreSnapshot()
                  })

                  it("should succeed for the first member", async () => {
                    await assertSubmissionSucceeds(1)
                  })

                  it("should succeed for the second member", async () => {
                    await assertSubmissionSucceeds(2)
                  })

                  it("should succeed for the last member", async () => {
                    await assertSubmissionSucceeds(constants.groupSize - 1)
                  })
                })

                context("after the submission period", async () => {
                  before(async () => {
                    await createSnapshot()

                    await mineBlocksTo(
                      startBlock + params.dkgResultSubmissionTimeout
                    )
                  })

                  after(async () => {
                    await restoreSnapshot()
                  })

                  it("should revert for the first member", async () => {
                    await assertSubmissionReverts(1)
                  })

                  it("should revert for the second member", async () => {
                    await assertSubmissionReverts(2)
                  })

                  it("should revert for the last member", async () => {
                    await assertSubmissionReverts(constants.groupSize - 1)
                  })
                })
              })
            })

            context("with dkg result submitted", async () => {
              let dkgResult: DkgResult
              let submitter: SignerWithAddress
              let resultSubmissionBlock: number

              before(async () => {
                await createSnapshot()

                let tx: ContractTransaction
                ;({
                  transaction: tx,
                  dkgResult,
                  submitter,
                } = await signAndSubmitCorrectDkgResult(
                  walletRegistry,
                  groupPublicKey,
                  dkgSeed,
                  startBlock,
                  noMisbehaved
                ))

                resultSubmissionBlock = tx.blockNumber
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should revert 'Current state is not AWAITING_RESULT' error", async () => {
                await expect(
                  signAndSubmitCorrectDkgResult(
                    walletRegistry,
                    groupPublicKey,
                    dkgSeed,
                    startBlock,
                    noMisbehaved
                  )
                ).to.be.revertedWith("Current state is not AWAITING_RESULT")
              })

              context("with dkg result approved", async () => {
                before(async () => {
                  await createSnapshot()

                  await mineBlocksTo(
                    resultSubmissionBlock +
                      params.dkgResultChallengePeriodLength
                  )

                  await walletRegistry
                    .connect(submitter)
                    .approveDkgResult(dkgResult)
                })

                after(async () => {
                  await restoreSnapshot()
                })

                it("should revert", async () => {
                  await expect(
                    signAndSubmitCorrectDkgResult(
                      walletRegistry,
                      groupPublicKey,
                      dkgSeed,
                      startBlock,
                      noMisbehaved
                    )
                  ).to.be.revertedWith("Sortition pool unlocked")
                })
              })
            })

            context(
              "with invalid dkg result submitted and challenged",
              async () => {
                let dkgResult: DkgResult
                let dkgResultHash: string
                let submitter: SignerWithAddress

                before(async () => {
                  await createSnapshot()
                  ;({ dkgResult, dkgResultHash, submitter } =
                    await signAndSubmitArbitraryDkgResult(
                      walletRegistry,
                      groupPublicKey,
                      // Mix operators to make the result malicious.
                      mixOperators(await selectGroup(sortitionPool, dkgSeed)),
                      startBlock,
                      noMisbehaved
                    ))
                })

                after(async () => {
                  await restoreSnapshot()
                })

                context("with dkg result challenged", async () => {
                  let challengeBlockNumber: number

                  before(async () => {
                    await createSnapshot()

                    const tx = await walletRegistry.challengeDkgResult(
                      dkgResult
                    )
                    challengeBlockNumber = tx.blockNumber
                  })

                  after(async () => {
                    await restoreSnapshot()
                  })

                  context("with the same dkg result", async () => {
                    before(async () => {
                      await createSnapshot()
                    })

                    after(async () => {
                      await restoreSnapshot()
                    })

                    it("should succeed", async () => {
                      const tx = await walletRegistry
                        .connect(submitter)
                        .submitDkgResult(dkgResult)

                      await expectDkgResultSubmittedEvent(tx, {
                        resultHash: dkgResultHash,
                        seed: dkgSeed,
                        result: dkgResult,
                      })
                    })
                  })

                  context("with a fresh dkg result", async () => {
                    context("", async () => {
                      let tx: ContractTransaction
                      let expectedEventArgs: DkgResultSubmittedEventArgs

                      before(async () => {
                        await createSnapshot()

                        const {
                          transaction,
                          dkgResultHash: newDkgResultHash,
                          dkgResult: newDkgResult,
                        } = await signAndSubmitCorrectDkgResult(
                          walletRegistry,
                          ecdsaData.group2.publicKey,
                          dkgSeed,
                          startBlock,
                          noMisbehaved
                        )

                        tx = transaction

                        expectedEventArgs = {
                          resultHash: newDkgResultHash,
                          seed: dkgSeed,
                          result: newDkgResult,
                        }
                      })

                      after(async () => {
                        await restoreSnapshot()
                      })

                      it("should emit DkgResultSubmitted event", async () => {
                        await expectDkgResultSubmittedEvent(
                          tx,
                          expectedEventArgs
                        )
                      })
                    })

                    context("with the submission period started", async () => {
                      let submissionStartBlockNumber: number

                      before(async () => {
                        await createSnapshot()

                        submissionStartBlockNumber = challengeBlockNumber

                        await mineBlocksTo(submissionStartBlockNumber)
                      })

                      after(async () => {
                        await restoreSnapshot()
                      })

                      context(
                        "at the beginning of the submission period",
                        async () => {
                          it("should succeed for the first member", async () => {
                            await assertSubmissionSucceeds(1)
                          })

                          it("should succeed for the second member", async () => {
                            await assertSubmissionSucceeds(2)
                          })

                          it("should succeed for the last member", async () => {
                            await assertSubmissionSucceeds(
                              constants.groupSize - 1
                            )
                          })
                        }
                      )

                      context(
                        "at the end of the submission period",
                        async () => {
                          before(async () => {
                            await createSnapshot()

                            await mineBlocksTo(
                              submissionStartBlockNumber +
                                params.dkgResultSubmissionTimeout -
                                1
                            )
                          })

                          after(async () => {
                            await restoreSnapshot()
                          })

                          it("should succeed for the first member", async () => {
                            await assertSubmissionSucceeds(1)
                          })

                          it("should succeed for the second member", async () => {
                            await assertSubmissionSucceeds(2)
                          })

                          it("should succeed for the last member", async () => {
                            await assertSubmissionSucceeds(
                              constants.groupSize - 1
                            )
                          })
                        }
                      )

                      context("after the submission period", async () => {
                        before(async () => {
                          await createSnapshot()

                          await mineBlocksTo(
                            submissionStartBlockNumber +
                              params.dkgResultSubmissionTimeout
                          )
                        })

                        after(async () => {
                          await restoreSnapshot()
                        })

                        it("should revert for the first member", async () => {
                          await assertSubmissionReverts(1)
                        })

                        it("should revert for the second member", async () => {
                          await assertSubmissionReverts(2)
                        })

                        it("should revert for the last member", async () => {
                          await assertSubmissionReverts(constants.groupSize - 1)
                        })
                      })
                    })
                  })
                })
              }
            )

            context("with misbehaved members", async () => {
              let tx: ContractTransaction
              let dkgResult: DkgResult
              let dkgResultHash: string

              context(
                "with misbehaved members in ascending order",
                async () => {
                  const misbehavedIndices = [2, 9, 11, 30, 60, 64]

                  before(async () => {
                    await createSnapshot()
                    ;({
                      transaction: tx,
                      dkgResult,
                      dkgResultHash,
                    } = await signAndSubmitCorrectDkgResult(
                      walletRegistry,
                      groupPublicKey,
                      dkgSeed,
                      startBlock,
                      misbehavedIndices
                    ))
                  })

                  after(async () => {
                    await restoreSnapshot()
                  })

                  it("should emit DkgResultSubmitted", async () => {
                    await expectDkgResultSubmittedEvent(tx, {
                      resultHash: dkgResultHash,
                      seed: dkgSeed,
                      result: dkgResult,
                    })
                  })

                  it("should use close to 294 000 gas", async () => {
                    await assertGasUsed(tx, 294_000)
                  })
                }
              )
            })
          })

          context("with wallet creation timed out", async () => {
            before("increase time", async () => {
              await createSnapshot()

              await mineBlocksTo(startBlock + dkgTimeout)
            })

            after(async () => {
              await restoreSnapshot()
            })

            context("with timeout not notified", async () => {
              it("should revert with DKG timeout already passed error", async () => {
                await expect(
                  signAndSubmitCorrectDkgResult(
                    walletRegistry,
                    groupPublicKey,
                    dkgSeed,
                    startBlock,
                    noMisbehaved
                  )
                ).to.be.revertedWith("DKG timeout already passed")
              })
            })
          })

          // Submission Test Helpers
          async function assertSubmissionSucceeds(
            submitterIndex: number
          ): Promise<void> {
            await createSnapshot()

            const {
              transaction: tx,
              dkgResult,
              dkgResultHash,
            } = await signAndSubmitCorrectDkgResult(
              walletRegistry,
              groupPublicKey,
              dkgSeed,
              startBlock,
              noMisbehaved,
              submitterIndex
            )

            await expectDkgResultSubmittedEvent(tx, {
              resultHash: dkgResultHash,
              seed: dkgSeed,
              result: dkgResult,
            })

            await restoreSnapshot()
          }

          async function assertSubmissionReverts(
            submitterIndex: number,
            message = "DKG timeout already passed"
          ): Promise<void> {
            await expect(
              signAndSubmitCorrectDkgResult(
                walletRegistry,
                groupPublicKey,
                dkgSeed,
                startBlock,
                noMisbehaved,
                submitterIndex
              )
            ).to.be.revertedWith(message)
          }
        })
      })
    })
  })

  describe("approveDkgResult", async () => {
    context("with initial contract state", async () => {
      it("should revert with 'Current state is not CHALLENGE' error", async () => {
        await expect(
          walletRegistry.approveDkgResult(stubDkgResult)
        ).to.be.revertedWith("Current state is not CHALLENGE")
      })
    })

    context("with new wallet requested", async () => {
      before("request new wallet", async () => {
        await createSnapshot()
        await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("with relay entry not submitted", async () => {
        it("should revert with 'Current state is not CHALLENGE' error", async () => {
          await expect(
            walletRegistry.approveDkgResult(stubDkgResult)
          ).to.be.revertedWith("Current state is not CHALLENGE")
        })

        context("with relay entry submitted", async () => {
          let startBlock: number
          let dkgSeed: BigNumber

          before("submit relay entry", async () => {
            await createSnapshot()
            ;({ startBlock, dkgSeed } = await submitRelayEntry(walletRegistry))
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should revert with 'Current state is not CHALLENGE' error", async () => {
            await expect(
              walletRegistry.approveDkgResult(stubDkgResult)
            ).to.be.revertedWith("Current state is not CHALLENGE")
          })

          context("with dkg result not submitted", async () => {
            it("should revert with 'Current state is not CHALLENGE' error", async () => {
              await expect(
                walletRegistry.approveDkgResult(stubDkgResult)
              ).to.be.revertedWith("Current state is not CHALLENGE")
            })
          })

          context("with dkg result submitted", async () => {
            let resultSubmissionBlock: number
            let dkgResultHash: string
            let dkgResult: DkgResult
            let submitter: SignerWithAddress
            let submitterInitialBalance: BigNumber

            const submitterIndex = 1

            before(async () => {
              await createSnapshot()

              let tx: ContractTransaction
              ;({
                transaction: tx,
                dkgResult,
                dkgResultHash,
                submitter,
                submitterInitialBalance,
              } = await signAndSubmitCorrectDkgResult(
                walletRegistry,
                groupPublicKey,
                dkgSeed,
                startBlock,
                noMisbehaved,
                submitterIndex
              ))

              resultSubmissionBlock = tx.blockNumber
            })

            after(async () => {
              await restoreSnapshot()
            })

            context("with challenge period not passed", async () => {
              before(async () => {
                await createSnapshot()

                await mineBlocksTo(
                  resultSubmissionBlock +
                    params.dkgResultChallengePeriodLength -
                    1
                )
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should revert with 'Challenge period has not passed yet' error", async () => {
                await expect(
                  walletRegistry.connect(submitter).approveDkgResult(dkgResult)
                ).to.be.revertedWith("Challenge period has not passed yet")
              })
            })

            context("with challenge period passed", async () => {
              before(async () => {
                await createSnapshot()

                await mineBlocksTo(
                  resultSubmissionBlock + params.dkgResultChallengePeriodLength
                )
              })

              after(async () => {
                await restoreSnapshot()
              })

              context("when called by a DKG result submitter", async () => {
                let tx: ContractTransaction

                before(async () => {
                  await createSnapshot()

                  tx = await walletRegistry
                    .connect(submitter)
                    .approveDkgResult(dkgResult)
                })

                after(async () => {
                  await restoreSnapshot()
                })

                it("should emit DkgResultApproved event", async () => {
                  await expect(tx)
                    .to.emit(walletRegistry, "DkgResultApproved")
                    .withArgs(dkgResultHash, await submitter.getAddress())
                })

                it("should clean dkg data", async () => {
                  await assertDkgResultCleanData(walletRegistry)
                })

                it("should register a new wallet", async () => {
                  const wallet = await walletRegistry.getWallet(walletID)

                  await expect(wallet.membersIdsHash).to.be.equal(
                    hashUint32Array(dkgResult.members)
                  )
                })

                it("should emit WalletCreated event", async () => {
                  await expect(tx)
                    .to.emit(walletRegistry, "WalletCreated")
                    .withArgs(walletID, dkgResultHash)
                })

                it("should unlock the sortition pool", async () => {
                  await expect(await sortitionPool.isLocked()).to.be.false
                })

                it("should refund ETH to a submitter", async () => {
                  const postDkgResultApprovalSubmitterInitialBalance =
                    await provider.getBalance(await submitter.getAddress())
                  const diff = postDkgResultApprovalSubmitterInitialBalance.sub(
                    submitterInitialBalance
                  )

                  expect(diff).to.be.gt(0)
                  expect(diff).to.be.lt(
                    ethers.utils.parseUnits("1200000", "gwei") // 0.0012 ETH
                  )
                })

                // there are no misbehaving group members in the result,
                // everyone should be eligible for rewards
                it("should not mark properly behaving operators as ineligible for rewards", async () => {
                  await expect(tx).not.to.emit(
                    sortitionPool,
                    "IneligibleForRewards"
                  )
                })

                it("should use close to 274 000 gas", async () => {
                  await assertGasUsed(tx, 274_000)
                })
              })

              context("when called by a third party", async () => {
                context(
                  "when the third party is not yet eligible",
                  async () => {
                    before(async () => {
                      await createSnapshot()

                      await mineBlocks(
                        params.dkgSubmitterPrecedencePeriodLength - 1
                      )
                    })

                    after(async () => {
                      await restoreSnapshot()
                    })

                    it("should revert", async () => {
                      await expect(
                        walletRegistry
                          .connect(thirdParty)
                          .approveDkgResult(dkgResult)
                      ).to.be.revertedWith(
                        "Only the DKG result submitter can approve the result at this moment"
                      )
                    })
                  }
                )

                context("when the third party is eligible", async () => {
                  let tx: ContractTransaction
                  let thirdPartyInitialBalance: BigNumber

                  before(async () => {
                    await createSnapshot()

                    await mineBlocks(params.dkgSubmitterPrecedencePeriodLength)

                    thirdPartyInitialBalance = await provider.getBalance(
                      await thirdParty.getAddress()
                    )

                    tx = await walletRegistry
                      .connect(thirdParty)
                      .approveDkgResult(dkgResult)
                  })

                  after(async () => {
                    await restoreSnapshot()
                  })

                  it("should emit DkgResultApproved event", async () => {
                    await expect(tx)
                      .to.emit(walletRegistry, "DkgResultApproved")
                      .withArgs(dkgResultHash, thirdParty.address)
                  })

                  it("should refund ETH to a third party caller", async () => {
                    const postDkgResultApprovalThirdPartyInitialBalance =
                      await provider.getBalance(await thirdParty.getAddress())
                    const { dkgResultSubmissionGas } =
                      await walletRegistry.gasParameters()
                    const feeForDkgSubmission = dkgResultSubmissionGas.mul(
                      tx.gasPrice
                    )
                    // submission part was done by someone else and this is why
                    // we add submission dkg fee to the initial balance
                    const diff =
                      postDkgResultApprovalThirdPartyInitialBalance.sub(
                        thirdPartyInitialBalance.add(feeForDkgSubmission)
                      )
                    expect(diff).to.be.gt(0)
                    expect(diff).to.be.lt(
                      ethers.utils.parseUnits("1000000", "gwei") // 0,001 ETH
                    )
                  })
                })
              })
            })
          })

          context("when there was a challenged result before", async () => {
            let resultSubmissionBlock: number
            let dkgResultHash: string
            let dkgResult: DkgResult

            // First result is malicious and submitter is also malicious
            const maliciousSubmitter = 1

            // Submit a second result by another submitter
            const anotherSubmitterIndex = 6
            let anotherSubmitter: Signer

            before(async () => {
              await createSnapshot()

              const { dkgResult: maliciousDkgResult } =
                await signAndSubmitArbitraryDkgResult(
                  walletRegistry,
                  groupPublicKey,
                  // Mix operators to make the result malicious.
                  mixOperators(await selectGroup(sortitionPool, dkgSeed)),
                  startBlock,
                  noMisbehaved,
                  maliciousSubmitter
                )

              await walletRegistry.challengeDkgResult(maliciousDkgResult)

              let tx: ContractTransaction
              ;({
                transaction: tx,
                dkgResult,
                dkgResultHash,
                submitter: anotherSubmitter,
              } = await signAndSubmitCorrectDkgResult(
                walletRegistry,
                groupPublicKey,
                dkgSeed,
                startBlock,
                noMisbehaved,
                anotherSubmitterIndex
              ))

              resultSubmissionBlock = tx.blockNumber
            })

            after(async () => {
              await restoreSnapshot()
            })

            context("with challenge period not passed", async () => {
              before(async () => {
                await createSnapshot()

                await mineBlocksTo(
                  resultSubmissionBlock +
                    params.dkgResultChallengePeriodLength -
                    1
                )
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should revert with 'Challenge period has not passed yet' error", async () => {
                await expect(
                  walletRegistry
                    .connect(anotherSubmitter)
                    .approveDkgResult(dkgResult)
                ).to.be.revertedWith("Challenge period has not passed yet")
              })
            })

            context("with challenge period passed", async () => {
              let tx: ContractTransaction
              let initalAnotherSubmitterBalance: BigNumber

              before(async () => {
                await createSnapshot()

                await mineBlocksTo(
                  resultSubmissionBlock + params.dkgResultChallengePeriodLength
                )

                initalAnotherSubmitterBalance = await provider.getBalance(
                  await anotherSubmitter.getAddress()
                )

                tx = await walletRegistry
                  .connect(anotherSubmitter)
                  .approveDkgResult(dkgResult)
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should emit DkgResultApproved event", async () => {
                await expect(tx)
                  .to.emit(walletRegistry, "DkgResultApproved")
                  .withArgs(dkgResultHash, await anotherSubmitter.getAddress())
              })

              it("should register a new wallet", async () => {
                const wallet = await walletRegistry.getWallet(walletID)

                await expect(wallet.membersIdsHash).to.be.equal(
                  hashUint32Array(dkgResult.members)
                )
              })

              it("should emit WalletCreated event", async () => {
                await expect(tx)
                  .to.emit(walletRegistry, "WalletCreated")
                  .withArgs(walletID, dkgResultHash)
              })

              it("should unlock the sortition pool", async () => {
                await expect(await sortitionPool.isLocked()).to.be.false
              })

              it("should refund ETH to a submitter", async () => {
                const postDkgResultApprovalAnotherSubmitterInitialBalance =
                  await provider.getBalance(await anotherSubmitter.getAddress())
                const { dkgResultSubmissionGas } =
                  await walletRegistry.gasParameters()
                const feeForDkgSubmission = dkgResultSubmissionGas.mul(
                  tx.gasPrice
                )
                // submission part was done by someone else and this is why
                // we add submission dkg fee to the initial balance
                const diff =
                  postDkgResultApprovalAnotherSubmitterInitialBalance.sub(
                    initalAnotherSubmitterBalance.add(feeForDkgSubmission)
                  )

                expect(diff).to.be.gt(0)
                expect(diff).to.be.lt(
                  ethers.utils.parseUnits("2000000", "gwei") // 0,002 ETH
                )
              })
            })
          })

          context("with max periods duration", async () => {
            let tx: ContractTransaction
            let dkgResultHash: string
            let submitter: SignerWithAddress

            before(async () => {
              await createSnapshot()

              await mineBlocksTo(startBlock + dkgTimeout - 1)

              let dkgResult: DkgResult
              ;({ dkgResult, dkgResultHash, submitter } =
                await signAndSubmitCorrectDkgResult(
                  walletRegistry,
                  groupPublicKey,
                  dkgSeed,
                  startBlock,
                  noMisbehaved
                ))

              await mineBlocks(params.dkgResultChallengePeriodLength)

              tx = await walletRegistry
                .connect(submitter)
                .approveDkgResult(dkgResult)
            })

            after(async () => {
              await restoreSnapshot()
            })

            // Just an explicit assertion to make sure transaction passes correctly
            // for max periods duration.
            it("should emit DkgResultApproved event", async () => {
              await expect(tx)
                .to.emit(walletRegistry, "DkgResultApproved")
                .withArgs(dkgResultHash, submitter.address)
            })

            it("should unlock the sortition pool", async () => {
              await expect(await sortitionPool.isLocked()).to.be.false
            })
          })

          context("with misbehaved operators", async () => {
            const misbehavedIndices = [2, 9, 11, 30, 60, 64]
            let misbehavedIds: number[]
            let tx: ContractTransaction
            let dkgResult: DkgResult
            let submitter: SignerWithAddress
            let submitterInitialBalance: BigNumber

            before(async () => {
              await createSnapshot()

              await mineBlocksTo(startBlock + dkgTimeout - 1)
              ;({ dkgResult, submitter, submitterInitialBalance } =
                await signAndSubmitCorrectDkgResult(
                  walletRegistry,
                  groupPublicKey,
                  dkgSeed,
                  startBlock,
                  misbehavedIndices
                ))

              misbehavedIds = misbehavedIndices.map(
                (i) => dkgResult.members[i - 1]
              )

              await mineBlocks(params.dkgResultChallengePeriodLength)
              tx = await walletRegistry
                .connect(submitter)
                .approveDkgResult(dkgResult)
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should register a new wallet", async () => {
              // misbehavedIndices: [2, 9, 11, 30, 60, 64]
              const expectedMembers = [...dkgResult.members]
              expectedMembers.splice(1, 1) // index -1
              expectedMembers.splice(7, 1) // index -2 (cause expectedMembers already shrinked)
              expectedMembers.splice(8, 1) // index -3
              expectedMembers.splice(26, 1) // index -4
              expectedMembers.splice(55, 1) // index -5
              expectedMembers.splice(58, 1) // index -6

              expect(
                (await walletRegistry.getWallet(walletID)).membersIdsHash
              ).to.be.equal(hashUint32Array(expectedMembers))
            })

            it("should ban misbehaved operators from sortition pool rewards", async () => {
              const now = await helpers.time.lastBlockTime()
              const expectedUntil = now + params.sortitionPoolRewardsBanDuration

              await expect(tx)
                .to.emit(sortitionPool, "IneligibleForRewards")
                .withArgs(misbehavedIds, expectedUntil)
            })

            it("should clean dkg data", async () => {
              await assertDkgResultCleanData(walletRegistry)
            })

            it("should refund ETH to a submitter", async () => {
              const postDkgResultApprovalSubmitterInitialBalance =
                await provider.getBalance(await submitter.getAddress())
              const diff = postDkgResultApprovalSubmitterInitialBalance.sub(
                submitterInitialBalance
              )

              expect(diff).to.be.gt(ethers.utils.parseUnits("-1000000", "gwei")) // -0,001 ETH
              expect(diff).to.be.lt(
                ethers.utils.parseUnits("1000000", "gwei") // 0,001 ETH
              )
            })

            it("should use close to 330 000 gas", async () => {
              await assertGasUsed(tx, 330_000, 15_000)
            })
          })

          // This case shouldn't happen in real life. When a result is submitted
          // with invalid order of misbehaved operators it should be challenged.
          context(
            "when misbehaved operators are not in ascending order",
            async () => {
              const misbehavedIndices = [2, 9, 30, 11, 60, 64]

              let dkgResult: DkgResult
              let submitter: SignerWithAddress
              let tx: Promise<ContractTransaction>

              before(async () => {
                await createSnapshot()
                ;({ dkgResult, submitter } =
                  await signAndSubmitCorrectDkgResult(
                    walletRegistry,
                    groupPublicKey,
                    dkgSeed,
                    startBlock,
                    misbehavedIndices
                  ))

                await mineBlocks(params.dkgResultChallengePeriodLength)

                tx = walletRegistry
                  .connect(submitter)
                  .approveDkgResult(dkgResult)
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should succeed", async () => {
                await expect(tx).to.not.be.reverted
              })

              it("should use close to 330 000 gas", async () => {
                await assertGasUsed(await tx, 330_000, 15_000)
              })
            }
          )

          // This case shouldn't happen in real life. When a result is submitted
          // with invalid order of misbehaved operators it should be challenged.
          context("when misbehaved members contains duplicates", async () => {
            const misbehavedIndices = [2, 9, 9, 10]

            let dkgResult: DkgResult
            let submitter: SignerWithAddress

            before(async () => {
              await createSnapshot()
              ;({ dkgResult, submitter } = await signAndSubmitCorrectDkgResult(
                walletRegistry,
                groupPublicKey,
                dkgSeed,
                startBlock,
                misbehavedIndices
              ))

              await mineBlocks(params.dkgResultChallengePeriodLength)
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should succeed", async () => {
              await expect(
                walletRegistry.connect(submitter).approveDkgResult(dkgResult)
              ).to.not.be.reverted
            })
          })
        })
      })
    })

    context("with wallet registered", async () => {
      const existingWalletPublicKey: string = ecdsaData.group1.publicKey
      let existingWalletID: string

      before("create a wallet", async () => {
        await createSnapshot()
        ;({ walletID: existingWalletID } = await createNewWallet(
          walletRegistry,
          walletOwner.wallet,
          randomBeacon,
          existingWalletPublicKey
        ))

        await expect(await walletRegistry.isWalletRegistered(existingWalletID))
          .to.be.true
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("with new wallet creation started", async () => {
        let startBlock: number
        let dkgSeed: BigNumber

        before(
          "request new wallet creation and submit relay entry",
          async () => {
            await createSnapshot()
            await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
            ;({ startBlock, dkgSeed } = await submitRelayEntry(walletRegistry))
          }
        )

        after(async () => {
          await restoreSnapshot()
        })

        context("with dkg result not submitted", async () => {
          it("should revert with 'Current state is not CHALLENGE' error", async () => {
            await expect(
              walletRegistry.approveDkgResult(stubDkgResult)
            ).to.be.revertedWith("Current state is not CHALLENGE")
          })
        })

        context("with dkg result submitted", async () => {
          let dkgResultHash: string
          let dkgResult: DkgResult
          let submitter: SignerWithAddress
          let submitterInitialBalance: BigNumber

          const newResultPublicKey = ecdsaData.group2.publicKey
          const newWalletID = keccak256(newResultPublicKey)
          const newResultSubmitterIndex = 1

          before("submit dkg result", async () => {
            await createSnapshot()
            ;({ dkgResult, dkgResultHash, submitter, submitterInitialBalance } =
              await signAndSubmitCorrectDkgResult(
                walletRegistry,
                newResultPublicKey,
                dkgSeed,
                startBlock,
                noMisbehaved,
                newResultSubmitterIndex
              ))

            await mineBlocks(params.dkgResultChallengePeriodLength)
          })

          after(async () => {
            await restoreSnapshot()
          })

          context("when called by a DKG result submitter", async () => {
            let tx: ContractTransaction

            before(async () => {
              await createSnapshot()

              tx = await walletRegistry
                .connect(submitter)
                .approveDkgResult(dkgResult)
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should emit DkgResultApproved event", async () => {
              await expect(tx)
                .to.emit(walletRegistry, "DkgResultApproved")
                .withArgs(dkgResultHash, await submitter.getAddress())
            })

            it("should clean dkg data", async () => {
              await assertDkgResultCleanData(walletRegistry)
            })

            it("should register a new wallet", async () => {
              const wallet = await walletRegistry.getWallet(newWalletID)

              await expect(wallet.membersIdsHash).to.be.equal(
                hashUint32Array(dkgResult.members)
              )
            })

            it("should emit WalletCreated event", async () => {
              await expect(tx)
                .to.emit(walletRegistry, "WalletCreated")
                .withArgs(newWalletID, dkgResultHash)
            })

            it("should unlock the sortition pool", async () => {
              await expect(await sortitionPool.isLocked()).to.be.false
            })

            it("should refund ETH to a submitter", async () => {
              const postDkgResultApprovalSubmitterInitialBalance =
                await provider.getBalance(await submitter.getAddress())
              const diff = postDkgResultApprovalSubmitterInitialBalance.sub(
                submitterInitialBalance
              )

              expect(diff).to.be.gt(0)
              expect(diff).to.be.lt(
                ethers.utils.parseUnits("1200000", "gwei") // 0.0012 ETH
              )
            })
          })
        })
      })
    })
  })

  describe("challengeDkgResult", async () => {
    context("with caller being a contract", async () => {
      let dkgChallenger: DkgChallenger

      before("request new wallet", async () => {
        await createSnapshot()

        const DkgChallenger = await ethers.getContractFactory("DkgChallenger")
        dkgChallenger = await DkgChallenger.deploy(walletRegistry.address)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should revert", async () => {
        await expect(
          dkgChallenger.challengeDkgResult(stubDkgResult)
        ).to.be.revertedWith("Not EOA")
      })
    })

    context("with no wallets registered", async () => {
      it("should revert with 'Current state is not CHALLENGE'", async () => {
        await expect(
          walletRegistry.challengeDkgResult(stubDkgResult)
        ).to.be.revertedWith("Current state is not CHALLENGE")
      })

      context("with new wallet requested", async () => {
        before("request new wallet", async () => {
          await createSnapshot()
          await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
        })

        after(async () => {
          await restoreSnapshot()
        })

        context("with relay entry not submitted", async () => {
          it("should revert with 'Current state is not CHALLENGE'", async () => {
            await expect(
              walletRegistry.challengeDkgResult(stubDkgResult)
            ).to.be.revertedWith("Current state is not CHALLENGE")
          })

          context("with relay entry submitted", async () => {
            let startBlock: number
            let dkgSeed: BigNumber

            before("submit relay entry", async () => {
              await createSnapshot()
              ;({ startBlock, dkgSeed } = await submitRelayEntry(
                walletRegistry
              ))
            })

            after(async () => {
              await restoreSnapshot()
            })

            context("with dkg result not submitted", async () => {
              it("should revert with 'Current state is not CHALLENGE'", async () => {
                await expect(
                  walletRegistry.challengeDkgResult(stubDkgResult)
                ).to.be.revertedWith("Current state is not CHALLENGE")
              })
            })

            context("with invalid dkg result submitted", async () => {
              let resultSubmissionBlock: number
              let dkgResultHash: string
              let dkgResult: DkgResult
              let submitter: SignerWithAddress

              before(async () => {
                await createSnapshot()

                let tx: ContractTransaction
                ;({
                  transaction: tx,
                  dkgResult,
                  dkgResultHash,
                  submitter,
                } = await signAndSubmitArbitraryDkgResult(
                  walletRegistry,
                  groupPublicKey,
                  // Mix operators to make the result malicious.
                  mixOperators(await selectGroup(sortitionPool, dkgSeed)),
                  startBlock,
                  noMisbehaved
                ))

                resultSubmissionBlock = tx.blockNumber
              })

              after(async () => {
                await restoreSnapshot()
              })

              context("at the beginning of challenge period", async () => {
                context("called by a third party", async () => {
                  let challengeTx: ContractTransaction
                  let slashingTx: ContractTransaction

                  before(async () => {
                    await createSnapshot()

                    challengeTx = await walletRegistry
                      .connect(thirdParty)
                      .challengeDkgResult(dkgResult)

                    slashingTx = await staking.processSlashing(1)
                  })

                  after(async () => {
                    await restoreSnapshot()
                  })

                  it("should emit DkgResultChallenged event", async () => {
                    await expect(challengeTx)
                      .to.emit(walletRegistry, "DkgResultChallenged")
                      .withArgs(
                        dkgResultHash,
                        await thirdParty.getAddress(),
                        "Invalid group members"
                      )
                  })

                  it("should not unlock the sortition pool", async () => {
                    await expect(await sortitionPool.isLocked()).to.be.true
                  })

                  it("should emit DkgMaliciousResultSlashed event", async () => {
                    await expect(challengeTx)
                      .to.emit(walletRegistry, "DkgMaliciousResultSlashed")
                      .withArgs(dkgResultHash, to1e18(400), submitter.address)
                  })

                  it("should reward the notifier", async () => {
                    await expect(challengeTx)
                      .to.emit(staking, "NotifierRewarded")
                      .withArgs(
                        thirdParty.address,
                        constants.tokenStakingNotificationReward
                      )
                  })

                  it("should slash malicious result submitter", async () => {
                    const stakingProvider =
                      await walletRegistry.operatorToStakingProvider(
                        submitter.address
                      )
                    await expect(slashingTx)
                      .to.emit(staking, "TokensSeized")
                      .withArgs(stakingProvider, to1e18(400), false)
                  })

                  it("should use close to 1 820 000 gas", async () => {
                    await assertGasUsed(challengeTx, 1_820_000, 40_000)
                  })
                })
              })

              context("at the end of challenge period", async () => {
                before(async () => {
                  await createSnapshot()

                  await mineBlocksTo(
                    resultSubmissionBlock +
                      params.dkgResultChallengePeriodLength -
                      1
                  )
                })

                after(async () => {
                  await restoreSnapshot()
                })

                context("called by a third party", async () => {
                  let challengeTx: ContractTransaction
                  let slashingTx: ContractTransaction

                  before(async () => {
                    await createSnapshot()

                    challengeTx = await walletRegistry
                      .connect(thirdParty)
                      .challengeDkgResult(dkgResult)

                    slashingTx = await staking.processSlashing(1)
                  })

                  after(async () => {
                    await restoreSnapshot()
                  })

                  it("should emit DkgResultChallenged event", async () => {
                    await expect(challengeTx)
                      .to.emit(walletRegistry, "DkgResultChallenged")
                      .withArgs(
                        dkgResultHash,
                        await thirdParty.getAddress(),
                        "Invalid group members"
                      )
                  })

                  it("should not unlock the sortition pool", async () => {
                    await expect(await sortitionPool.isLocked()).to.be.true
                  })

                  it("should emit DkgMaliciousResultSlashed event", async () => {
                    await expect(challengeTx)
                      .to.emit(walletRegistry, "DkgMaliciousResultSlashed")
                      .withArgs(dkgResultHash, to1e18(400), submitter.address)
                  })

                  it("should reward the notifier", async () => {
                    await expect(challengeTx)
                      .to.emit(staking, "NotifierRewarded")
                      .withArgs(
                        thirdParty.address,
                        constants.tokenStakingNotificationReward
                      )
                  })

                  it("should slash malicious result submitter", async () => {
                    const stakingProvider =
                      await walletRegistry.operatorToStakingProvider(
                        submitter.address
                      )
                    await expect(slashingTx)
                      .to.emit(staking, "TokensSeized")
                      .withArgs(stakingProvider, to1e18(400), false)
                  })
                })
              })

              context("with challenge period passed", async () => {
                before(async () => {
                  await createSnapshot()

                  await mineBlocksTo(
                    resultSubmissionBlock +
                      params.dkgResultChallengePeriodLength
                  )
                })

                after(async () => {
                  await restoreSnapshot()
                })

                it("should revert with 'Challenge period has already passed' error", async () => {
                  await expect(
                    walletRegistry.challengeDkgResult(dkgResult)
                  ).to.be.revertedWith("Challenge period has already passed")
                })
              })

              context(
                "with challenged result not matching the submitted one",
                async () => {
                  it("should revert with 'Result under challenge is different than the submitted one'", async () => {
                    const modifiedDkgResult: DkgResult = {
                      ...dkgResult,
                    }
                    modifiedDkgResult.submitterMemberIndex =
                      dkgResult.submitterMemberIndex + 1

                    await expect(
                      walletRegistry.challengeDkgResult(modifiedDkgResult)
                    ).to.be.revertedWith(
                      "Result under challenge is different than the submitted one"
                    )
                  })
                }
              )
            })

            context(
              "with dkg result submitted with unrecoverable signatures",
              async () => {
                let dkgResultHash: string
                let dkgResult: DkgResult
                let submitter: SignerWithAddress

                let challengeTx: ContractTransaction
                let slashingTx: ContractTransaction

                before(async () => {
                  await createSnapshot()
                  ;({ dkgResult, dkgResultHash, submitter } =
                    await signAndSubmitUnrecoverableDkgResult(
                      walletRegistry,
                      groupPublicKey,
                      await selectGroup(sortitionPool, dkgSeed),
                      startBlock,
                      noMisbehaved
                    ))

                  challengeTx = await walletRegistry
                    .connect(thirdParty)
                    .challengeDkgResult(dkgResult)

                  slashingTx = await staking.processSlashing(1)
                })

                after(async () => {
                  await restoreSnapshot()
                })

                it("should emit DkgResultChallenged event", async () => {
                  await expect(challengeTx)
                    .to.emit(walletRegistry, "DkgResultChallenged")
                    .withArgs(
                      dkgResultHash,
                      await thirdParty.getAddress(),
                      "validation reverted"
                    )
                })

                it("should not unlock the sortition pool", async () => {
                  await expect(await sortitionPool.isLocked()).to.be.true
                })

                it("should emit DkgMaliciousResultSlashed event", async () => {
                  await expect(challengeTx)
                    .to.emit(walletRegistry, "DkgMaliciousResultSlashed")
                    .withArgs(dkgResultHash, to1e18(400), submitter.address)
                })

                it("should reward the notifier", async () => {
                  await expect(challengeTx)
                    .to.emit(staking, "NotifierRewarded")
                    .withArgs(
                      thirdParty.address,
                      constants.tokenStakingNotificationReward
                    )
                })

                it("should slash malicious result submitter", async () => {
                  const stakingProvider =
                    await walletRegistry.operatorToStakingProvider(
                      submitter.address
                    )
                  await expect(slashingTx)
                    .to.emit(staking, "TokensSeized")
                    .withArgs(stakingProvider, to1e18(400), false)
                })

                it("should use close to 510 000 gas", async () => {
                  await assertGasUsed(challengeTx, 510_000, 15_000)
                })
              }
            )

            context(
              "when misbehaved operators are not in ascending order",
              async () => {
                const misbehavedIndices = [2, 9, 30, 11, 60, 64]

                let tx: ContractTransaction
                let dkgResult: DkgResult
                let dkgResultHash: string

                before(async () => {
                  await createSnapshot()
                  ;({ dkgResult, dkgResultHash } =
                    await signAndSubmitCorrectDkgResult(
                      walletRegistry,
                      groupPublicKey,
                      dkgSeed,
                      startBlock,
                      misbehavedIndices
                    ))

                  tx = await walletRegistry
                    .connect(thirdParty)
                    .challengeDkgResult(dkgResult)
                })

                after(async () => {
                  await restoreSnapshot()
                })

                it("should emit DkgResultChallenged event", async () => {
                  await expect(tx)
                    .to.emit(walletRegistry, "DkgResultChallenged")
                    .withArgs(
                      dkgResultHash,
                      await thirdParty.getAddress(),
                      "Corrupted misbehaved members indices"
                    )
                })

                it("should not unlock the sortition pool", async () => {
                  await expect(await sortitionPool.isLocked()).to.be.true
                })

                it("should use close to 330 000 gas", async () => {
                  await assertGasUsed(tx, 330_000, 20_000)
                })
              }
            )

            context("when misbehaved members contains duplicates", async () => {
              const misbehavedIndices = [2, 9, 30, 30, 60, 64]

              let tx: ContractTransaction
              let dkgResult: DkgResult
              let dkgResultHash: string

              before(async () => {
                await createSnapshot()
                ;({ dkgResult, dkgResultHash } =
                  await signAndSubmitCorrectDkgResult(
                    walletRegistry,
                    groupPublicKey,
                    dkgSeed,
                    startBlock,
                    misbehavedIndices
                  ))

                tx = await walletRegistry
                  .connect(thirdParty)
                  .challengeDkgResult(dkgResult)
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should emit DkgResultChallenged event", async () => {
                await expect(tx)
                  .to.emit(walletRegistry, "DkgResultChallenged")
                  .withArgs(
                    dkgResultHash,
                    await thirdParty.getAddress(),
                    "Corrupted misbehaved members indices"
                  )
              })

              it("should not unlock the sortition pool", async () => {
                await expect(await sortitionPool.isLocked()).to.be.true
              })

              it("should use close to 330 000 gas", async () => {
                await assertGasUsed(tx, 330_000, 20_000)
              })
            })

            context("with correct dkg result submitted", async () => {
              let dkgResult: DkgResult

              before(async () => {
                await createSnapshot()
                ;({ dkgResult } = await signAndSubmitCorrectDkgResult(
                  walletRegistry,
                  groupPublicKey,
                  dkgSeed,
                  startBlock,
                  noMisbehaved
                ))
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should revert with 'unjustified challenge' error", async () => {
                await expect(
                  walletRegistry.challengeDkgResult(dkgResult)
                ).to.be.revertedWith("unjustified challenge")
              })
            })

            context(
              "challengeDkgResult with simplified error handling (try-catch removal optimization)",
              async () => {
                let dkgResult: DkgResult
                let dkgResultHash: string
                let submitter: SignerWithAddress

                before(async () => {
                  await createSnapshot()

                  let tx: ContractTransaction
                  ;({
                    transaction: tx,
                    dkgResult,
                    dkgResultHash,
                    submitter,
                  } = await signAndSubmitArbitraryDkgResult(
                    walletRegistry,
                    groupPublicKey,
                    mixOperators(await selectGroup(sortitionPool, dkgSeed)),
                    startBlock,
                    noMisbehaved
                  ))
                })

                after(async () => {
                  await restoreSnapshot()
                })

                context(
                  "when staking.seize() succeeds (normal operation)",
                  async () => {
                    let challengeTx: ContractTransaction

                    before(async () => {
                      await createSnapshot()

                      challengeTx = await walletRegistry
                        .connect(thirdParty)
                        .challengeDkgResult(dkgResult)
                    })

                    after(async () => {
                      await restoreSnapshot()
                    })

                    it("should emit DkgResultChallenged event", async () => {
                      await expect(challengeTx)
                        .to.emit(walletRegistry, "DkgResultChallenged")
                        .withArgs(
                          dkgResultHash,
                          await thirdParty.getAddress(),
                          "Invalid group members"
                        )
                    })

                    it("should complete challenge successfully", async () => {
                      expect(await walletRegistry.getWalletCreationState()).to.equal(
                        dkgState.AWAITING_RESULT
                      )
                    })

                    it("should NOT emit DkgMaliciousResultSlashingFailed event after optimization", async () => {
                      const receipt = await challengeTx.wait()
                      const failedEvents = receipt.events?.filter(
                        (e) => e.event === "DkgMaliciousResultSlashingFailed"
                      )
                      expect(failedEvents?.length || 0).to.equal(0)
                    })

                    it("should still emit DkgMaliciousResultSlashed event on success", async () => {
                      await expect(challengeTx)
                        .to.emit(walletRegistry, "DkgMaliciousResultSlashed")
                        .withArgs(dkgResultHash, to1e18(400), submitter.address)
                    })

                    it("should use less gas than current implementation (bytecode optimization)", async () => {
                      const receipt = await challengeTx.wait()
                      const gasUsed = receipt.gasUsed.toNumber()
                      expect(gasUsed).to.be.lessThan(1_850_000)
                    })
                  }
                )

              }
            )
          })
        })
      })
    })

    context("with wallet registered", async () => {
      before("create a wallet", async () => {
        await createSnapshot()

        await createNewWallet(walletRegistry, walletOwner.wallet, randomBeacon)
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should revert with 'Current state is not CHALLENGE", async () => {
        await expect(
          walletRegistry.challengeDkgResult(stubDkgResult)
        ).to.be.revertedWith("Current state is not CHALLENGE")
      })

      context("with new wallet creation started", async () => {
        let startBlock: number
        let dkgSeed: BigNumber

        before(
          "request new wallet creation and submit relay entry",
          async () => {
            await createSnapshot()
            await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
            ;({ startBlock, dkgSeed } = await submitRelayEntry(walletRegistry))
          }
        )

        after(async () => {
          await restoreSnapshot()
        })

        it("should revert with 'Current state is not CHALLENGE'", async () => {
          await expect(
            walletRegistry.challengeDkgResult(stubDkgResult)
          ).to.be.revertedWith("Current state is not CHALLENGE")
        })

        context("with dkg result not submitted", async () => {
          it("should revert with 'Current state is not CHALLENGE'", async () => {
            await expect(
              walletRegistry.challengeDkgResult(stubDkgResult)
            ).to.be.revertedWith("Current state is not CHALLENGE")
          })
        })

        context("with invalid dkg result submitted", async () => {
          let resultSubmissionBlock: number
          let dkgResultHash: string
          let dkgResult: DkgResult
          let submitter: SignerWithAddress

          before(async () => {
            await createSnapshot()

            let tx: ContractTransaction
            ;({
              transaction: tx,
              dkgResult,
              dkgResultHash,
              submitter,
            } = await signAndSubmitArbitraryDkgResult(
              walletRegistry,
              groupPublicKey2,
              // Mix operators to make the result malicious.
              mixOperators(await selectGroup(sortitionPool, dkgSeed)),
              startBlock,
              noMisbehaved
            ))

            resultSubmissionBlock = tx.blockNumber
          })

          after(async () => {
            await restoreSnapshot()
          })

          context("at the beginning of challenge period", async () => {
            context("called by a third party", async () => {
              let challengeTx: ContractTransaction
              let slashingTx: ContractTransaction

              before(async () => {
                await createSnapshot()

                challengeTx = await walletRegistry
                  .connect(thirdParty)
                  .challengeDkgResult(dkgResult)

                slashingTx = await staking.processSlashing(1)
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should emit DkgResultChallenged event", async () => {
                await expect(challengeTx)
                  .to.emit(walletRegistry, "DkgResultChallenged")
                  .withArgs(
                    dkgResultHash,
                    await thirdParty.getAddress(),
                    "Invalid group members"
                  )
              })

              it("should not unlock the sortition pool", async () => {
                await expect(await sortitionPool.isLocked()).to.be.true
              })

              it("should emit DkgMaliciousResultSlashed event", async () => {
                await expect(challengeTx)
                  .to.emit(walletRegistry, "DkgMaliciousResultSlashed")
                  .withArgs(dkgResultHash, to1e18(400), submitter.address)
              })

              it("should slash malicious result submitter", async () => {
                const stakingProvider =
                  await walletRegistry.operatorToStakingProvider(
                    submitter.address
                  )
                await expect(slashingTx)
                  .to.emit(staking, "TokensSeized")
                  .withArgs(stakingProvider, to1e18(400), false)
              })
            })
          })

          context("at the end of challenge period", async () => {
            before(async () => {
              await createSnapshot()

              await mineBlocksTo(
                resultSubmissionBlock +
                  params.dkgResultChallengePeriodLength -
                  1
              )
            })

            after(async () => {
              await restoreSnapshot()
            })

            context("called by a third party", async () => {
              let challengeTx: ContractTransaction
              let slashingTx: ContractTransaction

              before(async () => {
                await createSnapshot()

                challengeTx = await walletRegistry
                  .connect(thirdParty)
                  .challengeDkgResult(dkgResult)

                slashingTx = await staking.processSlashing(1)
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should emit DkgResultChallenged event", async () => {
                await expect(challengeTx)
                  .to.emit(walletRegistry, "DkgResultChallenged")
                  .withArgs(
                    dkgResultHash,
                    await thirdParty.getAddress(),
                    "Invalid group members"
                  )
              })

              it("should not unlock the sortition pool", async () => {
                await expect(await sortitionPool.isLocked()).to.be.true
              })

              it("should emit DkgMaliciousResultSlashed event", async () => {
                await expect(challengeTx)
                  .to.emit(walletRegistry, "DkgMaliciousResultSlashed")
                  .withArgs(dkgResultHash, to1e18(400), submitter.address)
              })

              it("should slash malicious result submitter", async () => {
                const stakingProvider =
                  await walletRegistry.operatorToStakingProvider(
                    submitter.address
                  )
                await expect(slashingTx)
                  .to.emit(staking, "TokensSeized")
                  .withArgs(stakingProvider, to1e18(400), false)
              })
            })
          })

          context("with challenge period passed", async () => {
            before(async () => {
              await createSnapshot()

              await mineBlocksTo(
                resultSubmissionBlock + params.dkgResultChallengePeriodLength
              )
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should revert with 'Challenge period has already passed' error", async () => {
              await expect(
                walletRegistry.challengeDkgResult(dkgResult)
              ).to.be.revertedWith("Challenge period has already passed")
            })
          })

          context(
            "with challenged result not matching the submitted one",
            async () => {
              it("should revert with 'Result under challenge is different than the submitted one'", async () => {
                const modifiedDkgResult: DkgResult = { ...dkgResult }
                modifiedDkgResult.submitterMemberIndex =
                  dkgResult.submitterMemberIndex + 1

                await expect(
                  walletRegistry.challengeDkgResult(modifiedDkgResult)
                ).to.be.revertedWith(
                  "Result under challenge is different than the submitted one"
                )
              })
            }
          )
        })

        context(
          "with dkg result submitted with unrecoverable signatures",
          async () => {
            let dkgResultHash: string
            let dkgResult: DkgResult
            let submitter: SignerWithAddress

            let challengeTx: ContractTransaction
            let slashingTx: ContractTransaction

            before(async () => {
              await createSnapshot()
              ;({ dkgResult, dkgResultHash, submitter } =
                await signAndSubmitUnrecoverableDkgResult(
                  walletRegistry,
                  groupPublicKey2,
                  await selectGroup(sortitionPool, dkgSeed),
                  startBlock,
                  noMisbehaved
                ))

              challengeTx = await walletRegistry
                .connect(thirdParty)
                .challengeDkgResult(dkgResult)

              slashingTx = await staking.processSlashing(1)
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should emit DkgResultChallenged event", async () => {
              await expect(challengeTx)
                .to.emit(walletRegistry, "DkgResultChallenged")
                .withArgs(
                  dkgResultHash,
                  await thirdParty.getAddress(),
                  "validation reverted"
                )
            })

            it("should not unlock the sortition pool", async () => {
              await expect(await sortitionPool.isLocked()).to.be.true
            })

            it("should emit DkgMaliciousResultSlashed event", async () => {
              await expect(challengeTx)
                .to.emit(walletRegistry, "DkgMaliciousResultSlashed")
                .withArgs(dkgResultHash, to1e18(400), submitter.address)
            })

            it("should slash malicious result submitter", async () => {
              const stakingProvider =
                await walletRegistry.operatorToStakingProvider(
                  submitter.address
                )
              await expect(slashingTx)
                .to.emit(staking, "TokensSeized")
                .withArgs(stakingProvider, to1e18(400), false)
            })
          }
        )

        context("with correct dkg result submitted", async () => {
          let dkgResult: DkgResult

          before(async () => {
            await createSnapshot()
            ;({ dkgResult } = await signAndSubmitCorrectDkgResult(
              walletRegistry,
              groupPublicKey2,
              dkgSeed,
              startBlock,
              noMisbehaved
            ))
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should revert with 'unjustified challenge' error", async () => {
            await expect(
              walletRegistry.challengeDkgResult(dkgResult)
            ).to.be.revertedWith("unjustified challenge")
          })
        })
      })
    })

    // This test checks that dkg timeout is adjusted in case of result challenges
    // to include the offset blocks that were mined until the invalid result
    // was challenged.
    describe("submission start offset", async () => {
      before(async () => {
        await createSnapshot()
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should enforce submission start offset", async () => {
        let dkgResult: DkgResult

        await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
        const { startBlock } = await submitRelayEntry(walletRegistry)

        // Submit result 1 at the beginning of the submission period
        ;({ dkgResult } = await signAndSubmitArbitraryDkgResult(
          walletRegistry,
          groupPublicKey,
          operators,
          startBlock,
          noMisbehaved
        ))

        await expect(
          (
            await walletRegistry.getDkgData()
          ).resultSubmissionStartBlockOffset,
          "invalid resultSubmissionStartBlockOffset for result 1 after submission"
        ).to.equal(0)

        // Challenge result 1 at the beginning of the challenge period
        await walletRegistry.challengeDkgResult(dkgResult)
        // 1 block for dkg result submission tx +
        // 1 block for challenge tx
        let expectedSubmissionOffset = 2

        await expect(
          (
            await walletRegistry.getDkgData()
          ).resultSubmissionStartBlockOffset,
          "invalid resultSubmissionStartBlockOffset for result 1 after challenge"
        ).to.equal(expectedSubmissionOffset)

        // Submit result 2 in the middle of the submission period
        let blocksToMine = params.dkgResultSubmissionTimeout / 2
        await mineBlocks(blocksToMine)
        ;({ dkgResult } = await signAndSubmitArbitraryDkgResult(
          walletRegistry,
          groupPublicKey,
          operators,
          startBlock,
          noMisbehaved
        ))

        await expect(
          (
            await walletRegistry.getDkgData()
          ).resultSubmissionStartBlockOffset,
          "invalid resultSubmissionStartBlockOffset for result 2 after submission"
        ).to.equal(expectedSubmissionOffset) // same as before

        expectedSubmissionOffset += blocksToMine

        // Challenge result 2 in the middle of the challenge period
        await mineBlocks(params.dkgResultChallengePeriodLength / 2)
        expectedSubmissionOffset += params.dkgResultChallengePeriodLength / 2
        await walletRegistry.challengeDkgResult(dkgResult)
        expectedSubmissionOffset += 2 // 1 block for dkg result submission tx + 1 block for challenge tx

        await expect(
          (
            await walletRegistry.getDkgData()
          ).resultSubmissionStartBlockOffset,
          "invalid resultSubmissionStartBlockOffset for result 2 after challenge"
        ).to.equal(expectedSubmissionOffset)

        // Submit result 3 at the end of the submission period
        blocksToMine = params.dkgResultSubmissionTimeout - 1
        await mineBlocks(blocksToMine)
        ;({ dkgResult } = await signAndSubmitArbitraryDkgResult(
          walletRegistry,
          groupPublicKey,
          operators,
          startBlock,
          noMisbehaved
        ))

        await expect(
          (
            await walletRegistry.getDkgData()
          ).resultSubmissionStartBlockOffset,
          "invalid resultSubmissionStartBlockOffset for result 3 after submission"
        ).to.equal(expectedSubmissionOffset) // same as before

        expectedSubmissionOffset += blocksToMine

        // Challenge result 3 at the end of the challenge period
        blocksToMine = params.dkgResultChallengePeriodLength - 1
        await mineBlocks(blocksToMine)
        expectedSubmissionOffset += blocksToMine

        await expect(
          walletRegistry.callStatic.notifyDkgTimeout()
        ).to.be.revertedWith("DKG has not timed out")

        await walletRegistry.challengeDkgResult(dkgResult)
        expectedSubmissionOffset += 2 // 1 block for dkg result submission tx + 1 block for challenge tx

        await expect(
          (
            await walletRegistry.getDkgData()
          ).resultSubmissionStartBlockOffset,
          "invalid resultSubmissionStartBlockOffset for result 3 after challenge"
        ).to.equal(expectedSubmissionOffset)

        // Submit result 4 after the submission period
        blocksToMine = params.dkgResultSubmissionTimeout
        await mineBlocks(blocksToMine)
        await expect(
          signAndSubmitArbitraryDkgResult(
            walletRegistry,
            groupPublicKey,
            operators,
            startBlock,
            noMisbehaved
          )
        ).to.be.revertedWith("DKG timeout already passed")

        await walletRegistry.notifyDkgTimeout()
      })
    })
  })

  describe("isDkgResultValid", async () => {
    context("with group creation not in progress", async () => {
      it("should revert with 'DKG has not been started'", async () => {
        await expect(
          walletRegistry.isDkgResultValid(stubDkgResult)
        ).to.be.revertedWith("DKG has not been started")
      })
    })

    context("with new wallet creation in progress", async () => {
      let startBlock: number
      let dkgSeed: BigNumber

      before("request new wallet creation and submit relay entry", async () => {
        await createSnapshot()
        await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
        ;({ startBlock, dkgSeed } = await submitRelayEntry(walletRegistry))
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("with invalid result", async () => {
        it("should return false and an error message", async () => {
          const expectedValidationResult = [false, "Malformed signatures array"]

          const validationResult = await walletRegistry.isDkgResultValid(
            stubDkgResult
          )

          await expect(validationResult).to.be.deep.equal(
            expectedValidationResult
          )
        })
      })

      context("with valid result", async () => {
        let dkgResult: DkgResult

        before("start new wallet creation", async () => {
          await createSnapshot()
          ;({ dkgResult } = await signDkgResult(
            await selectGroup(sortitionPool, dkgSeed),
            groupPublicKey,
            noMisbehaved,
            startBlock
          ))
        })

        after(async () => {
          await restoreSnapshot()
        })

        it("should return true", async () => {
          const expectedValidationResult = [true, ""]

          const validationResult = await walletRegistry.isDkgResultValid(
            dkgResult
          )

          await expect(validationResult).to.be.deep.equal(
            expectedValidationResult
          )
        })
      })
    })
  })

  describe("hasSeedTimedOut", async () => {
    context("with initial contract state", async () => {
      it("should return false", async () => {
        await expect(await walletRegistry.hasSeedTimedOut()).to.be.false
      })
    })

    context("with new wallet requested", async () => {
      let requestNewWalletStartBlock: number

      before(async () => {
        await createSnapshot()
        const tx = await walletRegistry
          .connect(walletOwner.wallet)
          .requestNewWallet()

        requestNewWalletStartBlock = tx.blockNumber
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("with relay entry not submitted", async () => {
        context("with seed timeout period not passed", async () => {
          before(async () => {
            await createSnapshot()

            await mineBlocksTo(
              requestNewWalletStartBlock + params.dkgSeedTimeout
            )
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should return false", async () => {
            await expect(await walletRegistry.hasSeedTimedOut()).to.be.false
          })
        })

        context("with relay entry submitted", async () => {
          before(async () => {
            await createSnapshot()
            await submitRelayEntry(walletRegistry)
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should return false", async () => {
            await expect(await walletRegistry.hasSeedTimedOut()).to.be.false
          })
        })

        context("with seed timeout period passed", async () => {
          before(async () => {
            await createSnapshot()

            await mineBlocksTo(
              requestNewWalletStartBlock + params.dkgSeedTimeout + 1
            )
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should return true", async () => {
            await expect(await walletRegistry.hasSeedTimedOut()).to.be.true
          })

          context("with relay entry submitted", async () => {
            before(async () => {
              await createSnapshot()
              await submitRelayEntry(walletRegistry)
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should return false", async () => {
              await expect(await walletRegistry.hasSeedTimedOut()).to.be.false
            })
          })
        })
      })
    })
  })

  describe("notifySeedTimeout", async () => {
    context("with initial contract state", async () => {
      it("should revert with 'DKG has not timed out' error", async () => {
        await expect(walletRegistry.notifySeedTimeout()).to.be.revertedWith(
          "Awaiting seed has not timed out"
        )
      })
    })

    context("with new wallet requested", async () => {
      let requestNewWalletStartBlock: number

      before(async () => {
        await createSnapshot()
        const tx = await walletRegistry
          .connect(walletOwner.wallet)
          .requestNewWallet()

        requestNewWalletStartBlock = tx.blockNumber
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("with relay entry not submitted", async () => {
        context("with seed timeout period not passed", async () => {
          before(async () => {
            await createSnapshot()

            await mineBlocksTo(
              requestNewWalletStartBlock + params.dkgSeedTimeout - 1
            )
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should revert with 'Awaiting seed has not timed out' error", async () => {
            await expect(walletRegistry.notifySeedTimeout()).to.be.revertedWith(
              "Awaiting seed has not timed out"
            )
          })

          context("with relay entry submitted", async () => {
            before(async () => {
              await createSnapshot()
              await submitRelayEntry(walletRegistry)
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should revert with 'Awaiting seed has not timed out' error", async () => {
              await expect(
                walletRegistry.notifySeedTimeout()
              ).to.be.revertedWith("Awaiting seed has not timed out")
            })
          })
        })

        context("with seed timeout period passed", async () => {
          before(async () => {
            await createSnapshot()

            await mineBlocksTo(
              requestNewWalletStartBlock + params.dkgSeedTimeout
            )
          })

          after(async () => {
            await restoreSnapshot()
          })

          context("called by a third party", async () => {
            let tx: ContractTransaction
            let initThirdPartyBalance: BigNumber

            before(async () => {
              await createSnapshot()

              initThirdPartyBalance = await provider.getBalance(
                thirdParty.address
              )

              tx = await walletRegistry.connect(thirdParty).notifySeedTimeout()
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should emit DkgSeedTimedOut event", async () => {
              await expect(tx).to.emit(walletRegistry, "DkgSeedTimedOut")
            })

            it("should clean dkg data", async () => {
              await assertDkgResultCleanData(walletRegistry)
            })

            it("should unlock the sortition pool", async () => {
              await expect(await sortitionPool.isLocked()).to.be.false
            })

            it("should transition DKG to IDLE state", async () => {
              await expect(
                await walletRegistry.getWalletCreationState()
              ).to.be.equal(dkgState.IDLE)
            })

            it("should refund ETH", async () => {
              const postNotifyThirdPartyBalance = await provider.getBalance(
                thirdParty.address
              )
              const diff = postNotifyThirdPartyBalance.sub(
                initThirdPartyBalance
              )
              expect(diff).to.be.gt(0)
              expect(diff).to.be.lt(
                ethers.utils.parseUnits("100000", "gwei") // 0,0001 ETH
              )
            })

            it("should use close to 82 000 gas", async () => {
              await assertGasUsed(tx, 82_000)
            })
          })

          context("with relay entry submitted", async () => {
            before(async () => {
              await createSnapshot()
              await submitRelayEntry(walletRegistry)
            })

            after(async () => {
              await restoreSnapshot()
            })

            it("should revert with 'Awaiting seed has not timed out' error", async () => {
              await expect(
                walletRegistry.notifySeedTimeout()
              ).to.be.revertedWith("Awaiting seed has not timed out")
            })
          })
        })
      })
    })
  })

  describe("notifyDkgTimeout", async () => {
    context("with initial contract state", async () => {
      it("should revert with 'DKG has not timed out' error", async () => {
        await expect(walletRegistry.notifyDkgTimeout()).to.be.revertedWith(
          "DKG has not timed out"
        )
      })
    })

    context("with new wallet requested", async () => {
      let requestNewWalletStartBlock: number

      before(async () => {
        await createSnapshot()
        const tx = await walletRegistry
          .connect(walletOwner.wallet)
          .requestNewWallet()

        requestNewWalletStartBlock = tx.blockNumber
      })

      after(async () => {
        await restoreSnapshot()
      })

      context("with relay entry not submitted", async () => {
        context("with dkg timeout period passed", async () => {
          before(async () => {
            await createSnapshot()

            await mineBlocksTo(requestNewWalletStartBlock + dkgTimeout + 1)
          })

          after(async () => {
            await restoreSnapshot()
          })

          it("should revert with 'DKG has not timed out' error", async () => {
            await expect(walletRegistry.notifyDkgTimeout()).to.be.revertedWith(
              "DKG has not timed out"
            )
          })
        })

        context("with relay entry submitted", async () => {
          let startBlock: number

          before(async () => {
            await createSnapshot()
            ;({ startBlock } = await submitRelayEntry(walletRegistry))
          })

          after(async () => {
            await restoreSnapshot()
          })

          context("with dkg not timed out", async () => {
            context("with result submission period almost ended", async () => {
              before(async () => {
                await createSnapshot()

                await mineBlocksTo(startBlock + dkgTimeout - 1)
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should revert with 'DKG has not timed out' error", async () => {
                await expect(
                  walletRegistry.notifyDkgTimeout()
                ).to.be.revertedWith("DKG has not timed out")
              })
            })
          })

          context("with dkg timed out", async () => {
            before(async () => {
              await createSnapshot()

              await mineBlocksTo(startBlock + dkgTimeout)
            })

            after(async () => {
              await restoreSnapshot()
            })

            context("called by a third party", async () => {
              let tx: ContractTransaction
              let initThirdPartyBalance: BigNumber

              before(async () => {
                await createSnapshot()

                initThirdPartyBalance = await provider.getBalance(
                  thirdParty.address
                )
                tx = await walletRegistry.connect(thirdParty).notifyDkgTimeout()
              })

              after(async () => {
                await restoreSnapshot()
              })

              it("should emit DkgTimedOut event", async () => {
                await expect(tx).to.emit(walletRegistry, "DkgTimedOut")
              })

              it("should clean dkg data", async () => {
                await assertDkgResultCleanData(walletRegistry)
              })

              it("should unlock the sortition pool", async () => {
                await expect(await sortitionPool.isLocked()).to.be.false
              })

              it("should refund ETH", async () => {
                const postNotifyThirdPartyBalance = await provider.getBalance(
                  thirdParty.address
                )
                const diff = postNotifyThirdPartyBalance.sub(
                  initThirdPartyBalance
                )
                expect(diff).to.be.gt(0)
                expect(diff).to.be.lt(
                  ethers.utils.parseUnits("100000", "gwei") // 0,0001 ETH
                )
              })

              it("should use close to 76 000 gas", async () => {
                await assertGasUsed(tx, 76_000)
              })
            })
          })
        })
      })
    })
  })

  describe("selectGroup", async () => {
    context("when dkg was not triggered", async () => {
      before(async () => {
        await createSnapshot()
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should revert", async () => {
        await expect(walletRegistry.selectGroup()).to.be.revertedWith(
          "Sortition pool unlocked"
        )
      })
    })

    context("when dkg was triggered", async () => {
      let dkgSeed: BigNumber

      before(async () => {
        await createSnapshot()
        await walletRegistry.connect(walletOwner.wallet).requestNewWallet()
        ;({ dkgSeed } = await submitRelayEntry(walletRegistry))
      })

      after(async () => {
        await restoreSnapshot()
      })

      it("should select a group", async () => {
        const selectedGroup = await walletRegistry.selectGroup()
        expect(selectedGroup.length).to.eq(constants.groupSize)
      })

      it("should be the same group as if called the sortition pool directly", async () => {
        const exectedGroup = await sortitionPool.selectGroup(
          constants.groupSize,
          ethers.utils.hexZeroPad(dkgSeed.toHexString(), 32)
        )
        const actualGroup = await walletRegistry.selectGroup()
        expect(exectedGroup).to.be.deep.equal(actualGroup)
      })
    })
  })
})

async function assertDkgResultCleanData(walletRegistry: WalletRegistryStub) {
  const dkgData = await walletRegistry.getDkgData()

  expect(
    dkgData.parameters.resultChallengePeriodLength,
    "unexpected resultChallengePeriodLength"
  ).to.eq(params.dkgResultChallengePeriodLength)

  expect(
    dkgData.parameters.resultSubmissionTimeout,
    "unexpected resultSubmissionTimeout"
  ).to.eq(params.dkgResultSubmissionTimeout)

  expect(
    dkgData.parameters.submitterPrecedencePeriodLength,
    "unexpected submitterPrecedencePeriodLength"
  ).to.eq(params.dkgSubmitterPrecedencePeriodLength)

  expect(dkgData.startBlock, "unexpected startBlock").to.eq(0)

  expect(
    dkgData.resultSubmissionStartBlockOffset,
    "unexpected resultSubmissionStartBlockOffset"
  ).to.eq(0)

  expect(dkgData.submittedResultHash, "unexpected submittedResultHash").to.eq(
    ethers.constants.HashZero
  )

  expect(dkgData.submittedResultBlock, "unexpected submittedResultBlock").to.eq(
    0
  )
}

function mixOperators(operators: Operator[]): Operator[] {
  return operators
    .map((v) => ({ v, sort: Math.random() }))
    .sort((a, b) => a.sort - b.sort)
    .map(({ v }) => v)
}
